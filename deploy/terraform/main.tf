terraform {
  required_version = ">= 1.13, < 2.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.0.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  type    = string
  default = "tireshop-513806"
}
variable "region" {
  type    = string
  default = "asia-southeast1"
}
variable "deploy_services" {
  type    = bool
  default = false
}
variable "images" {
  description = "Artifact Registry image references pinned by sha256, keyed by catalog and requests."
  type        = map(string)
  default     = {}
  validation {
    condition = !var.deploy_services || (
      toset(keys(var.images)) == toset(["catalog", "requests"]) &&
      alltrue([for image in values(var.images) : can(regex("^[a-z0-9-]+-docker.pkg.dev/.+@sha256:[a-f0-9]{64}$", image))])
    )
    error_message = "Deployment requires catalog and requests Artifact Registry digests."
  }
}
variable "secret_versions" {
  type    = object({ catalog = string, requests = string, ca = string })
  default = { catalog = "1", requests = "1", ca = "1" }
}

locals {
  services = toset(["catalog", "requests"])
  apis     = toset(["run.googleapis.com", "artifactregistry.googleapis.com", "secretmanager.googleapis.com", "iam.googleapis.com"])
}

resource "google_project_service" "api" {
  for_each           = local.apis
  service            = each.value
  disable_on_destroy = false
}
resource "google_artifact_registry_repository" "api" {
  location      = var.region
  repository_id = "tireshop"
  format        = "DOCKER"
  depends_on    = [google_project_service.api]
  lifecycle { prevent_destroy = true }
}
resource "google_service_account" "api" {
  for_each     = local.services
  account_id   = "tireshop-${each.key}"
  display_name = "Tireshop ${each.key} runtime"
  depends_on   = [google_project_service.api]
}
resource "google_secret_manager_secret" "runtime" {
  for_each  = toset(["catalog", "requests", "ca"])
  secret_id = "tireshop-${each.key}"
  replication {
    auto {}
  }
  depends_on = [google_project_service.api]
  lifecycle { prevent_destroy = true }
}
# Values/versions are intentionally created by the operator outside Terraform.
resource "google_secret_manager_secret_iam_member" "password" {
  for_each  = local.services
  secret_id = google_secret_manager_secret.runtime[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api[each.key].email}"
}
resource "google_secret_manager_secret_iam_member" "ca" {
  for_each  = local.services
  secret_id = google_secret_manager_secret.runtime["ca"].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api[each.key].email}"
}
resource "google_cloud_run_v2_service" "api" {
  for_each            = var.deploy_services ? local.services : toset([])
  name                = "tireshop-${each.key}"
  location            = var.region
  deletion_protection = true
  ingress             = "INGRESS_TRAFFIC_ALL"
  template {
    service_account                  = google_service_account.api[each.key].email
    max_instance_request_concurrency = 20
    timeout                          = "60s"
    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }
    containers {
      image = var.images[each.key]
      ports { container_port = 10000 }
      resources {
        limits            = { cpu = "1", memory = "512Mi" }
        cpu_idle          = true
        startup_cpu_boost = true
      }
      dynamic "env" {
        for_each = {
          APP_ENV                            = "production"
          HTTP_ADDR                          = "0.0.0.0:10000"
          PGHOST                             = "pg-2f7b3aad-snorchello-5783.g.aivencloud.com"
          PGPORT                             = "22201"
          PGDATABASE                         = "defaultdb"
          PGUSER                             = "defaultdb_${each.key}_login"
          PGSSLROOTCERT                      = "/etc/tireshop-ca/ca.pem"
          PGPOOL_MAX_CONNS                   = "4"
          APP_ORIGINS                        = "https://tireshop-3x8p.onrender.com"
          "${upper(each.key)}_PASSWORD_FILE" = "/etc/tireshop-password/password"
        }
        content {
          name  = env.key
          value = env.value
        }
      }
      volume_mounts {
        name       = "password"
        mount_path = "/etc/tireshop-password"
      }
      volume_mounts {
        name       = "ca"
        mount_path = "/etc/tireshop-ca"
      }
      startup_probe {
        http_get { path = "/readyz" }
        period_seconds    = 5
        timeout_seconds   = 2
        failure_threshold = 24
      }
      liveness_probe {
        http_get { path = "/healthz" }
        period_seconds = 30
      }
    }
    volumes {
      name = "password"
      secret {
        secret = google_secret_manager_secret.runtime[each.key].secret_id
        items {
          version = var.secret_versions[each.key]
          path    = "password"
        }
      }
    }
    volumes {
      name = "ca"
      secret {
        secret = google_secret_manager_secret.runtime["ca"].secret_id
        items {
          version = var.secret_versions.ca
          path    = "ca.pem"
        }
      }
    }
  }
  depends_on = [google_secret_manager_secret_iam_member.password, google_secret_manager_secret_iam_member.ca]
}
# Render's static proxy cannot supply Google IAM identity tokens.
# Public invocation does not grant application admin permissions.
resource "google_cloud_run_v2_service_iam_member" "public" {
  for_each = google_cloud_run_v2_service.api
  project  = var.project_id
  location = var.region
  name     = each.value.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
output "registry" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.api.repository_id}"
}
output "service_urls" {
  value = { for name, service in google_cloud_run_v2_service.api : name => service.uri }
}

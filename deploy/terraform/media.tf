variable "enable_public_media" {
  description = "Create a public bucket containing only published product images."
  type        = bool
  default     = false
}

resource "google_project_service" "storage" {
  count              = var.enable_public_media ? 1 : 0
  service            = "storage.googleapis.com"
  disable_on_destroy = false
}

resource "google_storage_bucket" "media" {
  count                       = var.enable_public_media ? 1 : 0
  name                        = "${var.project_id}-product-media"
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "inherited"
  force_destroy               = false
  depends_on                  = [google_project_service.storage]

  lifecycle { prevent_destroy = true }
}

# Published product photos only. No application credentials or source documents.
# Object Viewer allows public reads and listing; anonymous writes are not granted.
resource "google_storage_bucket_iam_member" "public_media" {
  count  = var.enable_public_media ? 1 : 0
  bucket = google_storage_bucket.media[0].name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}

output "media_bucket" {
  value = var.enable_public_media ? google_storage_bucket.media[0].name : null
}

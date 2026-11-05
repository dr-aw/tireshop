# Mocked plans only: no GCP credentials or resource creation.
mock_provider "google" {}

run "foundation_has_no_running_services" {
  command = plan
  assert {
    condition     = length(google_cloud_run_v2_service.api) == 0
    error_message = "First apply must not start services before secrets/migrations are ready."
  }
}

run "deployment_rejects_missing_images" {
  command = plan
  variables {
    deploy_services = true
  }
  expect_failures = [var.images]
}

run "deployment_uses_pinned_images" {
  command = plan
  variables {
    deploy_services = true
    images = {
      catalog  = "asia-southeast1-docker.pkg.dev/tireshop-513806/tireshop/catalog@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      requests = "asia-southeast1-docker.pkg.dev/tireshop-513806/tireshop/requests@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    }
  }
  assert {
    condition     = length(google_cloud_run_v2_service.api) == 2
    error_message = "A complete release requires both independent API services."
  }
}

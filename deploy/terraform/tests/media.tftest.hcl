mock_provider "google" {}

run "media_disabled_by_default" {
  command = plan
  assert {
    condition     = length(google_storage_bucket.media) == 0
    error_message = "Existing deployments must opt into public media explicitly."
  }
}

run "public_photos_have_no_anonymous_write_access" {
  command = plan
  variables {
    enable_public_media = true
  }
  assert {
    condition = (
      google_storage_bucket.media[0].uniform_bucket_level_access &&
      !google_storage_bucket.media[0].force_destroy &&
      google_storage_bucket_iam_member.public_media[0].role == "roles/storage.objectViewer" &&
      google_storage_bucket_iam_member.public_media[0].member == "allUsers"
    )
    error_message = "Published media requires uniform access, read-only public IAM and no forced deletion."
  }
}

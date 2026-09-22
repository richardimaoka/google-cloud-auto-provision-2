output "project_id" {
  description = "Value for GOOGLE_CLOUD_PROJECT_ID in .env."
  value       = google_project.this.project_id
}

output "project_number" {
  description = "Numeric ID of the project."
  value       = google_project.this.number
}

output "automation_sa_email" {
  description = "Value for AUTOMATION_SA_EMAIL in .env."
  value       = google_service_account.automation.email
}

output "webapp_image_repo" {
  description = "Artifact Registry repo to push the webapp image to before deploying."
  value       = "${var.region}-docker.pkg.dev/${google_project.this.project_id}/${google_artifact_registry_repository.webapp.repository_id}"
}

output "webapp_url" {
  description = "URL of the deployed Cloud Run webapp service."
  value       = google_cloud_run_v2_service.webapp.uri
}

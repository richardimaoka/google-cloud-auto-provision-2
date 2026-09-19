output "project_id" {
  description = "Value for BIGQUERY_PROJECT_ID in .env."
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

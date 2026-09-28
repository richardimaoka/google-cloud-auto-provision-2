output "project_id" {
  description = "Value for GOOGLECLOUD_TEST_CONTROL_PROJECT in .env.integration."
  value       = google_project.this.project_id
}

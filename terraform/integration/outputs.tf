output "project_id" {
  description = "Value for GOOGLECLOUD_TEST_CONTROL_PROJECT in .env.integration."
  value       = google_project.this.project_id
}

output "bigquery_dataset" {
  description = "Value for GOOGLECLOUD_TEST_BIGQUERY_DATASET in .env.integration."
  value       = module.progress_table.dataset_id
}

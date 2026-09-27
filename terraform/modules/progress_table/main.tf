# The auto_provision dataset and its project_creation_progress table, shared by
# the production root (../../production) and the integration root
# (../../integration), so the schema is defined once.

# Holds the application's records.
resource "google_bigquery_dataset" "app" {
  project = var.project_id
  # googlecloud.ProgressDatasetID in Go; change both together.
  dataset_id = "auto_provision"
  location   = var.location
}

# The current progress of each project creation request: one row per
# request_id, updated in place as the request moves through its states.
# BigQuery doesn't enforce keys, so the app keeps request_id unique itself.
resource "google_bigquery_table" "project_creation_progress" {
  project    = var.project_id
  dataset_id = google_bigquery_dataset.app.dataset_id
  table_id   = "project_creation_progress"

  schema = jsonencode([
    { name = "request_id", type = "STRING", mode = "REQUIRED", description = "Caller-issued ID of the request." },
    { name = "revision", type = "INT64", mode = "REQUIRED", description = "Goes up when a failed request is amended." },
    { name = "state", type = "STRING", mode = "REQUIRED", description = "Current step, like creating_project." },
    { name = "status", type = "STRING", mode = "REQUIRED", description = "in_progress, error or success." },
    { name = "project_id", type = "STRING", mode = "NULLABLE", description = "Created project's ID. NULL while creating_project." },
    { name = "display_name", type = "STRING", mode = "REQUIRED", description = "Requested display name." },
    { name = "requester", type = "STRING", mode = "REQUIRED", description = "Requester's email." },
    { name = "developer_group", type = "STRING", mode = "REQUIRED", description = "Developer group's email." },
    { name = "error_message", type = "STRING", mode = "NULLABLE", description = "Why the current step failed, when status is error." },
    { name = "created_at", type = "TIMESTAMP", mode = "REQUIRED", description = "When the request was recorded." },
    { name = "updated_at", type = "TIMESTAMP", mode = "REQUIRED", description = "When the row last changed." },
  ])
}

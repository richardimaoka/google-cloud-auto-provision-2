# The control project the googlecloud integration tests use, and the BigQuery
# table they write to. It has its own state and variables, so running it can't
# touch production. The tests run with your own ADC credentials, which need
# access to the dataset (for example as the project's owner).

locals {
  org_id    = startswith(var.parent, "organizations/") ? trimprefix(var.parent, "organizations/") : null
  folder_id = startswith(var.parent, "folders/") ? trimprefix(var.parent, "folders/") : null
}

# Created by hand and imported, like production (see README). Terraform is the
# source of truth for its name, billing account and parent from then on.
resource "google_project" "this" {
  project_id      = var.project_id
  name            = var.project_name != "" ? var.project_name : var.project_id
  billing_account = var.billing_account
  org_id          = local.org_id
  folder_id       = local.folder_id

  # The provider's default, spelled out: terraform destroy never deletes the project.
  deletion_policy = "PREVENT"
}

resource "google_project_service" "bigquery" {
  project = google_project.this.project_id
  service = "bigquery.googleapis.com"

  # Leave the API on if this configuration is destroyed; other things may use it.
  disable_on_destroy = false
}

module "progress_table" {
  source     = "../modules/progress_table"
  project_id = google_project.this.project_id
  location   = var.region

  depends_on = [google_project_service.bigquery]
}

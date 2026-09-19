locals {
  org_id    = startswith(var.parent, "organizations/") ? trimprefix(var.parent, "organizations/") : null
  folder_id = startswith(var.parent, "folders/") ? trimprefix(var.parent, "folders/") : null

  apis = [
    "serviceusage.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "cloudbilling.googleapis.com",
    "iam.googleapis.com",
    "cloudidentity.googleapis.com",
    "bigquery.googleapis.com",
  ]
}

# The project is created by hand and imported (see README), because the
# provider counts API calls against it and so it has to exist before Terraform
# runs. Once imported, Terraform is the source of truth for these settings.
resource "google_project" "this" {
  project_id      = var.project_id
  name            = var.project_name != "" ? var.project_name : var.project_id
  billing_account = var.billing_account
  org_id          = local.org_id
  folder_id       = local.folder_id
}

resource "google_project_service" "apis" {
  for_each = toset(local.apis)

  project = google_project.this.project_id
  service = each.value

  # Leave the API on if this configuration is destroyed; other things may use it.
  disable_on_destroy = false
}

resource "google_service_account" "automation" {
  project      = google_project.this.project_id
  account_id   = var.automation_sa_id
  display_name = "Auto-provision automation"

  depends_on = [google_project_service.apis]
}

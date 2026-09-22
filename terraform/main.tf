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
    "run.googleapis.com",
    "artifactregistry.googleapis.com",
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

# Holds the webapp's container images. Push with:
#   docker push <region>-docker.pkg.dev/<project_id>/webapp/webapp:<tag>
resource "google_artifact_registry_repository" "webapp" {
  project       = google_project.this.project_id
  location      = var.region
  repository_id = "webapp"
  format        = "DOCKER"

  depends_on = [google_project_service.apis]
}

# The Cloud Run service for the webapp package (see ../webapp, ../cmd/webapp).
resource "google_cloud_run_v2_service" "webapp" {
  project  = google_project.this.project_id
  name     = "webapp"
  location = var.region

  template {
    service_account = google_service_account.automation.email

    containers {
      image = "us-docker.pkg.dev/cloudrun/container/hello"
    }
  }

  depends_on = [google_project_service.apis]

  lifecycle {
    ignore_changes = [template[0].containers[0].image]
  }
}

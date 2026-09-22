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
    "iap.googleapis.com",
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
  project     = google_project.this.project_id
  name        = "webapp"
  location    = var.region
  iap_enabled = true

  template {
    service_account = google_service_account.automation.email

    containers {
      # Placeholder until the real image is pushed and deployed (see README); ignore_changes below keeps it from being reverted.
      image = "us-docker.pkg.dev/cloudrun/container/hello"
    }
  }

  depends_on = [google_project_service.apis]

  lifecycle {
    ignore_changes = [template[0].containers[0].image]
  }
}

# Lets IAP itself invoke the service. Regular IAM invoker checks stay on, so
# only this grant (plus whatever else you explicitly add) can call it.
resource "google_cloud_run_v2_service_iam_member" "iap_invoker" {
  project  = google_project.this.project_id
  location = google_cloud_run_v2_service.webapp.location
  name     = google_cloud_run_v2_service.webapp.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:service-${google_project.this.number}@gcp-sa-iap.iam.gserviceaccount.com"

  depends_on = [google_project_service.apis]
}

# Lets var.iap_access_domain sign in through IAP to reach the service.
# Equivalent of:
#   gcloud iap web add-iam-policy-binding --resource-type=cloud-run \
#     --service=webapp --region=<region> \
#     --member=domain:<iap_access_domain> --role=roles/iap.httpsResourceAccessor
resource "google_iap_web_cloud_run_service_iam_member" "domain_access" {
  project                = google_project.this.project_id
  location               = google_cloud_run_v2_service.webapp.location
  cloud_run_service_name = google_cloud_run_v2_service.webapp.name
  role                   = "roles/iap.httpsResourceAccessor"
  member                 = "domain:${var.iap_access_domain}"

  depends_on = [google_project_service.apis]
}

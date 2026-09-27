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
    "secretmanager.googleapis.com",
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

# Holds webapp/oauth-client.md's downloaded OAuth client JSON. Terraform only
# creates the secret itself; add the real value by hand once, out-of-band:
#   gcloud secrets versions add webapp-oauth-client --data-file=secrets/client_secret.json
# It's never in this repo's source or in Terraform state, same as
# secrets/client_secret.json itself.
resource "google_secret_manager_secret" "oauth_client" {
  project   = google_project.this.project_id
  secret_id = "webapp-oauth-client"

  replication {
    auto {}
  }

  depends_on = [google_project_service.apis]
}

# Lets the webapp's own runtime identity read the secret mounted into it below.
resource "google_secret_manager_secret_iam_member" "oauth_client_accessor" {
  project   = google_project.this.project_id
  secret_id = google_secret_manager_secret.oauth_client.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.automation.email}"
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
      args  = ["-oauth-client=/secrets/client_secret.json"]

      volume_mounts {
        name       = "oauth-client"
        mount_path = "/secrets"
      }
    }

    volumes {
      name = "oauth-client"
      secret {
        secret = google_secret_manager_secret.oauth_client.secret_id
        items {
          path    = "client_secret.json"
          version = "latest"
        }
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_secret_manager_secret_iam_member.oauth_client_accessor,
  ]

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

# Holds the application's records. The webapp and worker read and write it as
# the automation service account.
resource "google_bigquery_dataset" "app" {
  project = google_project.this.project_id
  # Hardcoded in webapp as bigQueryDataset; change both together.
  dataset_id = "auto_provision"
  location   = var.region

  depends_on = [google_project_service.apis]
}

# The current progress of each project creation request: one row per
# request_id, updated in place as the request moves through its states.
# BigQuery doesn't enforce keys, so the app keeps request_id unique itself.
resource "google_bigquery_table" "project_creation_progress" {
  project    = google_project.this.project_id
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

# Lets the automation service account read and write the app's tables...
resource "google_bigquery_dataset_iam_member" "app_editor" {
  project    = google_project.this.project_id
  dataset_id = google_bigquery_dataset.app.dataset_id
  role       = "roles/bigquery.dataEditor"
  member     = "serviceAccount:${google_service_account.automation.email}"
}

# ...and run the query jobs that do it.
resource "google_project_iam_member" "automation_bigquery_job_user" {
  project = google_project.this.project_id
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:${google_service_account.automation.email}"
}

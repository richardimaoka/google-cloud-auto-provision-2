# The BigQuery table the googlecloud integration tests write to, in a control
# project made for them. It has its own state and variables, so running it can't
# touch production. The tests run with your own ADC credentials, which need
# access to the dataset (for example as the project's owner).

terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.0"
    }
  }
}

provider "google" {
  project = var.project_id

  # Count API calls against var.project_id itself, like the production root.
  user_project_override = true
  billing_project       = var.project_id
}

resource "google_project_service" "bigquery" {
  project = var.project_id
  service = "bigquery.googleapis.com"

  # Leave the API on if this configuration is destroyed; other things may use it.
  disable_on_destroy = false
}

module "progress_table" {
  source     = "../modules/progress_table"
  project_id = var.project_id
  location   = var.region

  depends_on = [google_project_service.bigquery]
}

variable "project_id" {
  description = "Control project for the integration tests: GOOGLECLOUD_TEST_CONTROL_PROJECT."
  type        = string
}

variable "region" {
  description = "Location of the dataset."
  type        = string
  default     = "asia-northeast1"
}

output "bigquery_dataset" {
  description = "Value for GOOGLECLOUD_TEST_BIGQUERY_DATASET in .env.integration."
  value       = module.progress_table.dataset_id
}

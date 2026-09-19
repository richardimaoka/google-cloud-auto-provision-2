terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id

  # Count API calls against var.project_id itself, instead of whatever quota
  # project ADC currently has, so the target doesn't change with ADC.
  user_project_override = true
  billing_project       = var.project_id
}

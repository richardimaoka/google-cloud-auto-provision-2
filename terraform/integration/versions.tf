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

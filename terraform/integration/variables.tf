variable "project_id" {
  description = "ID of the integration test control project: GOOGLECLOUD_TEST_CONTROL_PROJECT. It is created by hand first, then imported (see README)."
  type        = string
}

variable "project_name" {
  description = "Display name of the project. Defaults to project_id."
  type        = string
  default     = ""
}

variable "billing_account" {
  description = "ID of the billing account to link, like 012345-6789AB-CDEF01. See `gcloud billing accounts list`."
  type        = string
}

variable "parent" {
  description = "Parent of the project, \"folders/123\" or \"organizations/456\". Must match where the project was created."
  type        = string

  validation {
    condition     = can(regex("^(folders|organizations)/[0-9]+$", var.parent))
    error_message = "parent must be \"folders/<id>\" or \"organizations/<id>\"."
  }
}

variable "region" {
  description = "Location of the dataset."
  type        = string
  default     = "asia-northeast1"
}

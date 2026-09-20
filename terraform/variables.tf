variable "project_id" {
  description = "ID of the project. It is created by hand first, then imported (see README)."
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
  description = "Parent of the project, \"folders/123\" or \"organizations/456\" (same format as PARENT_RESOURCE). Must match where the project was created. Leave empty if there is no Google Cloud organization."
  type        = string
  default     = ""

  validation {
    condition     = var.parent == "" || can(regex("^(folders|organizations)/[0-9]+$", var.parent))
    error_message = "parent must be empty, \"folders/<id>\" or \"organizations/<id>\"."
  }
}

variable "automation_sa_id" {
  description = "Account ID (the part before @) of the automation service account."
  type        = string
  default     = "provision-test"
}

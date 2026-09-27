# Terraform: integration test project

The googlecloud integration tests use their own control project, never
production. This configuration manages that project the same way
`../production` manages the production one: you create it by hand, import it,
and from then on Terraform is the source of truth for its name, billing
account and parent. It also creates the BigQuery table the tests write to,
from the same module as production (`../modules/progress_table`). It has its
own state and `terraform.tfvars`, so it can't touch production.

## One-time setup

Create the project and link billing by hand, as in `../production/README.md`,
so you pick the organization, project and billing account deliberately:

```sh
gcloud projects create <project_id> [--organization=<id> | --folder=<id>]
gcloud billing projects link <project_id> --billing-account=<billing_account>
gcloud services enable serviceusage.googleapis.com cloudresourcemanager.googleapis.com \
  cloudbilling.googleapis.com --project <project_id>
```

Then import it. Set `parent` in `terraform.tfvars` to what you passed to
`gcloud projects create`, or Terraform will try to move the project.

```sh
cp terraform.tfvars.example terraform.tfvars   # set project_id, billing_account, parent
terraform init
terraform import google_project.this <project_id>
terraform plan
terraform apply
```

Put `terraform output project_id` in `GOOGLECLOUD_TEST_CONTROL_PROJECT` and
`terraform output bigquery_dataset` in `GOOGLECLOUD_TEST_BIGQUERY_DATASET`, in
`../../.env.integration`.

## Day to day

Change the project's name, billing account or parent only here, through
`terraform plan` and `terraform apply`, never in the Cloud Console: the next
apply would change it back. `terraform destroy` removes the dataset and table
but never the project (`deletion_policy = "PREVENT"`).

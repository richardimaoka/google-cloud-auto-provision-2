# Terraform

Manages a Google Cloud project: the project itself (name, parent, billing
account link), the APIs the app uses, and the automation service account.
After the one-time setup below, Terraform is the source of truth for all of
these.

All API calls are counted against the project itself (`billing_project` is set
to `project_id`), not against whatever project ADC points at. That means the
project has to exist before Terraform runs, so it is created by hand once and
then imported.

## One-time setup

Find your billing account ID with `gcloud billing accounts list`, then:

```sh
gcloud projects create <project_id> [--organization=<id> | --folder=<id>]
gcloud billing projects link <project_id> --billing-account=<billing_account>
gcloud services enable serviceusage.googleapis.com cloudresourcemanager.googleapis.com \
  cloudbilling.googleapis.com --project <project_id>
```

Terraform needs those three APIs on the project to read and manage it, so
enable them before the first run. Set `parent` in `terraform.tfvars` to match
what you passed to `gcloud projects create`, or Terraform will try to move the
project.

The account you authenticate as needs permission to manage the project (for
example Owner), including `serviceusage.services.use`, and
`roles/billing.user` on the billing account.

```sh
gcloud auth application-default login
cp terraform.tfvars.example terraform.tfvars   # set project_id, billing_account
terraform init
terraform import google_project.this <project_id>
terraform plan
terraform apply
```

## Day to day

Run `terraform plan` and `terraform apply`. Changing `billing_account` or
`project_name` in `terraform.tfvars` changes the real project.

State is kept locally in `terraform.tfstate` and is git-ignored. Move it to a
remote backend before anyone else works on this.

`terraform output` gives the values for `AUTOMATION_SA_EMAIL` and
`BIGQUERY_PROJECT_ID` in `.env`. Afterwards, point ADC at the project with
`gcloud auth application-default set-quota-project <project_id>`.

`terraform destroy` will try to delete the project. If it can't, delete the
project in the Cloud Console.

## Not managed by Terraform

Terraform can't create an OAuth client for a regular web app. Set it up by hand
in the Cloud Console: see [../docs/oauth-client.md](../webapp/oauth-client.md).

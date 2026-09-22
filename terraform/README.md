# Terraform

Before the first `terraform apply`, you should manually create a Google Cloud
project and link it to a billing account. That way, you make sure you chose
the correct Google Cloud organization, project, and billing account.
Otherwise, letting Terraform create a project might pick up authentication
from ADC, which might not be what you want, especially if you frequently
switch among different Google logins. You then import the project into
Terraform state, so it becomes the source of truth for it going forward.

All API calls are counted against the project itself (`billing_project` is set
to `project_id`), not against whatever project ADC points at, so the project
has to exist before Terraform can even run.

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
`GOOGLE_CLOUD_PROJECT_ID` in `.env`. Afterwards, point ADC at the project with
`gcloud auth application-default set-quota-project <project_id>`.

`terraform destroy` will try to delete the project. If it can't, delete the
project in the Cloud Console.

## Deploying the webapp

`terraform apply` creates the Cloud Run service with a placeholder image
(Terraform can't reference an image that hasn't been pushed yet). To deploy
the real one, from the repository root:

`GOOGLE_CLOUD_PROJECT_ID` and `REGION` in `.env` (see `.env.example`) give
`project_id` and `region`, so you don't have to fill those in by hand:

```sh
set -a
. ./.env
set +a
project_id=$GOOGLE_CLOUD_PROJECT_ID
region=$REGION

gcloud auth configure-docker $region-docker.pkg.dev
docker build -t $region-docker.pkg.dev/$project_id/webapp/webapp:latest .
docker push $region-docker.pkg.dev/$project_id/webapp/webapp:latest
gcloud run deploy webapp --project $project_id --region $region \
  --image $region-docker.pkg.dev/$project_id/webapp/webapp:latest
```

`terraform output webapp_image_repo` and `terraform output webapp_url` give
the repo and the service URL. `terraform apply` afterwards leaves the deployed
image alone (`ignore_changes` on the container image in main.tf).

The service currently requires IAM `roles/run.invoker` to reach it — nothing
grants that yet, so it isn't publicly reachable until the webapp has its own
sign-in and that's added deliberately (`google_cloud_run_v2_service_iam_member`
with `member = "allUsers"`).

## Not managed by Terraform

Terraform can't create an OAuth client for a regular web app. Set it up by hand
in the Cloud Console: see [../webapp/oauth-client.md](../webapp/oauth-client.md).

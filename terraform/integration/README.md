# Terraform: integration test project

The googlecloud integration tests write to a `project_creation_progress` table
in a separate control project, never production. This configuration creates
just that dataset and table there, from the same module as production
(`../modules/progress_table`), with its own state and `terraform.tfvars`:

```sh
cp terraform.tfvars.example terraform.tfvars   # set project_id
terraform init
terraform apply
```

Put its project ID in `GOOGLECLOUD_TEST_CONTROL_PROJECT` and
`terraform output bigquery_dataset` in `GOOGLECLOUD_TEST_BIGQUERY_DATASET`, in
`../../.env.integration`.

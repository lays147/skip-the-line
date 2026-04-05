resource "aws_ecrpublic_repository" "app" {
  repository_name = local.project_name

  catalog_data {
    description       = "skip-the-line: GitHub webhook → Slack DM notification service"
    operating_systems = ["Linux"]
    architectures     = ["x86-64"]
  }
}

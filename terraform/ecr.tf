resource "aws_ecrpublic_repository" "app" {
  repository_name = local.project_name

  catalog_data {
    description       = "skip-the-line: GitHub webhook → Slack DM notification service"
    operating_systems = ["Linux"]
    architectures     = ["x86-64"]
    about_text        = "Skip The Line delivers real-time Slack DMs for pull request activity — so reviewers never miss a review request and authors always know the moment someone approves or leaves feedback. Sends notifications for review requests, review submissions, and comments. By using this service your organization may reduce Mean Time to Merge (MTTM) by around 40%. GitHub sends a signed webhook payload to this service. The service validates the signature, routes the event by type and action, resolves the relevant recipients from a subscriber registry, and sends each one a Slack DM."
    usage_text        = "See https://github.com/lays147/skip-the-line for complete documentation, including deployment instructions, environment variables, Kubernetes manifests, and GitHub webhook setup. Start with the main README.md for quickstart and Deployment.md for production setup."
  }
}

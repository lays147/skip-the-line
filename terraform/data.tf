data "aws_iam_openid_connect_provider" "github_oidc" {
  url = "https://${local.github.oidc_domain}"
}


resource "unionai_secret" "some_secret" {
  name    = "SOME_SECRET"
  project = "nelson"
  domain  = "development"
  value   = 12345
}

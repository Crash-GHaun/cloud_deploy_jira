# Configure the Google Cloud provider
terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.0"
    }
  }
}

variable "gcp_service_list" {
  description = "List of GCP Services to enable (WIP)"
  type = list(string)
  default = [
    "pubsub.googleapis.com",
    "clouddeploy.googleapis.com",
    "cloudbuild.googleapis.com"
  ]
}

# Enable Services (Work in Progress)
resource "google_project_service" "project" {
  for_each = toset(var.gcp_service_list)
  project = var.project_id
  service = each.key

  timeouts {
    create = "30m"
    update = "40m"
  }

  disable_on_destroy = false
}

# Create a Pub/Sub topic to receive JIRA notifications
resource "google_pubsub_topic" "jira_notifications" {
  name = "jira-notifications"
  project = var.project_id
}

# Create a Pub/Sub topic to receive JIRA notifications
resource "google_pubsub_topic" "deploy_notifications" {
  name = "clouddeploy-operations"
  project = var.project_id
}

resource "google_artifact_registry_repository" "random-date-app" {
  location      = "us-central1"
  repository_id = "random-date-app"
  description   = "Docker repo for random-date-app"
  format        = "DOCKER"
}

# Create a Cloud Deploy pipeline
resource "google_clouddeploy_delivery_pipeline" "primary" {
  name        = "jira-triggered-pipeline"
  project = var.project_id
  location    = var.region
  description = "Pipeline triggered by JIRA notifications"

  serial_pipeline {
    stages {
      target_id = google_clouddeploy_target.primary.name
      profiles = ["example-profile"] # Replace with your Cloud Deploy profile name
    }
  }
}

# Create a Cloud Run service
resource "google_cloud_run_v2_service" "main" {
  name     = "random-date-service"
  project = var.project_id
  location = var.region
  ingress = "INGRESS_TRAFFIC_ALL"

  template {
    containers {
      # We add a dummy image here to get the service created
      image = "us-docker.pkg.dev/cloudrun/container/hello"
    }
  }
}

# Create a Cloud Deploy target
resource "google_clouddeploy_target" "primary" {
  name     = "primary-target"
  project = var.project_id
  location = "us-central1"
  #location = var.region
  require_approval = false # Set to true if you want manual approval for deployments

  # Configure your deployment target (Cloud Run)
  run {
    location = "projects/{var.project_id}/locations/{var.location}"
  }
  depends_on = [ google_cloud_run_v2_service.main ]
}

//Create CloudBuild SA
resource "google_service_account" "cloudbuild_service_account" {
  account_id   = "cloudbuild-sa"
  display_name = "cloudbuild-sa"
  description  = "Cloud build service account"
}

resource "google_project_iam_member" "act_as" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.cloudbuild_service_account.email}"
}

resource "google_project_iam_member" "logs_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cloudbuild_service_account.email}"
}

#This isn't perfect because you have to connect the repo first
#Not sure how to do this in terraform yet TODO: @Ghaun
# Create a Cloud Build trigger
resource "google_cloudbuild_trigger" "build-cloudrun-deploy" {
  name        = "random-date-build-trigger"
  location = "global"
  service_account = google_service_account.cloudbuild_service_account.id
  github {
    owner = var.github_owner
    name = var.github_repo
    push {
      branch = "main"
    }
  }

  filename = "CloudBuild/buildCloudRun.yaml" # Path to your Cloud Build configuration file
}

resource "google_storage_bucket" "function_bucket" {
  name = "${var.project_id}-gcf-source"
  location = "US"
  uniform_bucket_level_access = true 
}

data "archive_file" "receiveJira" {
  type = "zip"
  output_path = "/tmp/function-recieve-jira.zip"
  source_dir = "CloudFunctions/recieveJiraNotification/"
}

data "archive_file" "sendJira" {
  type = "zip"
  output_path = "/tmp/function-send-jira.zip"
  source_dir = "CloudFunctions/sendJiraNotification/"
}

resource "google_storage_bucket_object" "recieveJira" {
  name = "function-recieve-jira.zip"
  bucket = google_storage_bucket.function_bucket.name
  source = data.archive_file.receiveJira.output_path
}

resource "google_storage_bucket_object" "sendJira" {
  name = "function-send-jira.zip"
  bucket = google_storage_bucket.function_bucket.name
  source = data.archive_file.sendJira.output_path
}

# Create a Cloud Function to trigger Cloud Deploy
resource "google_cloudfunctions2_function" "recieve-jira" {
  name    = "recieve-jira"
  project = var.project_id
  location = var.region

 build_config {
    entry_point = "deployTrigger"
    runtime     = "go122" # Or your preferred runtime
 source {
      storage_source {
        bucket = google_storage_bucket.function_bucket.name # Replace with your bucket name
        object = google_storage_bucket_object.recieveJira.name # Replace with your source code object
      }
    }
  }

  service_config {
    all_traffic_on_latest_revision = true
    available_memory               = "256M" # Adjust as needed
    ingress_settings               = "ALLOW_ALL"
    timeout_seconds                = 60 # Adjust as needed
  }

  event_trigger {
    event_type = "google.cloud.pubsub.topic.v1.messagePublished"
    retry_policy = "RETRY_POLICY_RETRY"
    trigger_region = var.region
    pubsub_topic = google_pubsub_topic.jira_notifications.id
  }
}

# Create a Cloud Function to send deployment updates to JIRA
resource "google_cloudfunctions2_function" "send-Jira" {
  name    = "send-Jira"
  project = var.project_id
  location = var.region

  build_config {
    entry_point = "updateJira"
    runtime     = "go122" # Or your preferred runtime
 source {
      storage_source {
        bucket = google_storage_bucket.function_bucket.name # Replace with your bucket name
        object = google_storage_bucket_object.sendJira.name # Replace with your source code object
      }
    }
  }

  service_config {
    all_traffic_on_latest_revision = true
    available_memory               = "256M" # Adjust as needed
    ingress_settings               = "ALLOW_ALL"
    timeout_seconds                = 60 # Adjust as needed
  }

  event_trigger {
    event_type = "google.cloud.pubsub.topic.v1.messagePublished"
    retry_policy = "RETRY_POLICY_RETRY"
    trigger_region = var.region
    pubsub_topic = google_pubsub_topic.deploy_notifications.id
  }
}
# Configure the Google Cloud provider
terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.0"
    }
  }
}

# Replace with your Google Cloud project ID
project_id = "your-project-id"

# Replace with your preferred region
region = "us-central1"

# Create a Pub/Sub topic to receive JIRA notifications
resource "google_pubsub_topic" "jira_notifications" {
  name = "jira-notifications"
  project = project_id
}

# Create a Cloud Deploy pipeline
resource "google_clouddeploy_delivery_pipeline" "primary" {
  name        = "jira-triggered-pipeline"
  project = project_id
  location    = region
  description = "Pipeline triggered by JIRA notifications"

  serial_pipeline {
    stages {
      target_id = google_clouddeploy_target.primary.id
      profiles = ["example-profile"] # Replace with your Cloud Deploy profile name
    }
  }
}

# Create a Cloud Deploy target
resource "google_clouddeploy_target" "primary" {
  name     = "primary-target"
  project = project_id
  location = region
  require_approval = false # Set to true if you want manual approval for deployments

  # Configure your deployment target (Cloud Run)
  cloud_run {
    service = google_cloud_run_v2_service.main.name
  }
}

# Create a Cloud Run service
resource "google_cloud_run_v2_service" "main" {
  name     = "random-date-service"
  project = project_id
  location = region

  template {
    containers {
      image = "us-central1-docker.pkg.dev/${project_id}/your-repo/random-date-app"
    }
  }
}

# Create a Cloud Build trigger
resource "google_cloudbuild_trigger" "main" {
  name        = "random-date-build-trigger"
  project = project_id
  location = region

  # Configure the trigger to build the Cloud Run service
  github {
    owner = "your-github-username"
    name  = "your-github-repo"
    push {
      branch = "^main$" # Trigger on pushes to the main branch
    }
  }

  filename = "cloudbuild.yaml" # Path to your Cloud Build configuration file
}

# Create a Cloud Function to trigger Cloud Deploy
resource "google_cloudfunctions2_function" "deploy_trigger" {
  name    = "deploy-trigger"
  project = project_id
  location = region

 build_config {
    entry_point = "process_jira_notification"
    runtime     = "nodejs16" # Or your preferred runtime
 source {
      storage_source {
        bucket = "your-source-bucket" # Replace with your bucket name
        object = "deploy-trigger-source.zip" # Replace with your source code object
      }
    }
  }

  service_config {
    all_traffic_on_latest_revision = true
    available_memory               = "256M" # Adjust as needed
    ingress_settings               = "ALLOW_ALL"
    timeout_seconds                = 60 # Adjust as needed

    event_trigger {
      event_type = "google.cloud.pubsub.topic.v1.messagePublished"
      retry_policy = "RETRY_POLICY_RETRY"
      trigger_region = region
      pubsub_topic = google_pubsub_topic.jira_notifications.id
    }
  }
}

# Create a Cloud Function to send deployment updates to JIRA
resource "google_cloudfunctions2_function" "jira_update" {
  name    = "jira-update"
  project = project_id
  location = region

  build_config {
    entry_point = "update_jira"
    runtime     = "nodejs16" # Or your preferred runtime
 source {
      storage_source {
        bucket = "your-source-bucket" # Replace with your bucket name
        object = "jira-update-source.zip" # Replace with your source code object
      }
    }
  }

  service_config {
    all_traffic_on_latest_revision = true
    available_memory               = "256M" # Adjust as needed
    ingress_settings               = "ALLOW_ALL"
    timeout_seconds                = 60 # Adjust as needed

    event_trigger {
      event_type = "google.cloud.deploy.topic.v1.messagePublished"
      retry_policy = "RETRY_POLICY_RETRY"
      trigger_region = region
      pubsub_topic = "projects/your-project-id/topics/clouddeploy-service-notifications" # This is the default Cloud Deploy notification topic
    }
  }
}
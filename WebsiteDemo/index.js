const { PubSub } = require('@google-cloud/pubsub');
const express = require('express');
const path = require('path');

const app = express();
const port = 8080;

// Replace with your actual project ID and subscription names
const projectId = 'crash-demo-env';
const subscriptionNames = [
  'build_notifications_subscription',
  'deploy-commands-subscription',
  'clouddeploy-operations-subscription',
  'clouddeploy-approvals-subscription'
];
const timeout = 60;

const pubsubClient = new PubSub({ projectId });

// Store messages per subscription
let messages = {
  'build_notifications_subscription': [],
  'deploy-commands-subscription': [],
  'clouddeploy-operations-subscription': [],
  'clouddeploy-approvals-subscription': []
};

// Function to pull messages from each subscription
async function pullMessages(pubSubClient, subscriptionName) {
  const subscription = pubSubClient.subscription(subscriptionName);

  const messageHandler = message => {
    console.log(`Received message ${message.id}:`);
    console.log(`\tData: ${message.data}`);
    console.log(`\tAttributes: ${JSON.stringify(message.attributes)}`);

    messages[subscriptionName].push({
      id: message.id,
      data: message.data.toString(),
      attributes: message.attributes
    });

    message.ack();
  };

  subscription.on('message', messageHandler);

  setTimeout(() => {
    subscription.removeListener('message', messageHandler);
  }, timeout * 1000);
}

// Main function to pull messages and serve static files
async function main() {
  setInterval(async () => {
    for (const subscriptionName of subscriptionNames) {
      await pullMessages(pubsubClient, subscriptionName);
    }
  }, 5000);

  app.use(express.static(path.join(__dirname, 'public')));

  // API endpoint to send messages to the frontend
  app.get('/messages', (req, res) => {
    res.json(messages);
  });

  // Endpoint to clear all messages
  app.post('/clear-messages', (req, res) => {
    // Clear the messages from all subscriptions
    messages = {
      'build_notifications_subscription': [],
      'deploy-commands-subscription': [],
      'clouddeploy-operations-subscription': [],
      'clouddeploy-approvals-subscription': []
    };
    console.log("All messages cleared.");
    res.sendStatus(200); // Respond with success
  });

  app.listen(port, () => {
    console.log(`Server is listening on port ${port}`);
  });
}

main().catch(console.error);
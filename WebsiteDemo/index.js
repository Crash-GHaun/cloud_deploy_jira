const { PubSub } = require('@google-cloud/pubsub');
const express = require('express');
const path = require('path');
const bodyParser = require('body-parser');

const app = express();
const port = 8080;
const topicName = 'clouddeploy-approvals';

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
  app.use(bodyParser.json());

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

  app.post('/send-message', async (req, res) => {
    const { message } = req.body;
  
    try {
      // Ensure the message is not empty
      if (!message) {
        return res.status(400).send('Message cannot be empty.');
      }
      const topic = pubsubClient.topic(topicName);
      
      // Create the message object with optional attributes
      const messageObject = {
        data: Buffer.from(message),
        // Optionally, you can include attributes like this:
        // attributes: {
        //   key: 'value' // Add any attributes you need
        // }
      };
  
      // Publish the message to the Pub/Sub topic
      const messageId = await topic.publish(messageObject);
      
      console.log(`Message sent with ID: ${messageId}`);
      
      // Send a success response
      res.status(200).send('Message published.');
    } catch (error) {
      console.error('Error publishing message:', error);
      
      // Send an error response
      res.status(500).send('Failed to publish message.'); 
    }
  });

  app.listen(port, () => {
    console.log(`Server is listening on port ${port}`);
  });

  
}

main().catch(console.error);
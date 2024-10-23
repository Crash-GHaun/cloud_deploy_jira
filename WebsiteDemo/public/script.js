let lastMessageData = {}; // Store last fetched messages to compare updates

    // Function to update the messages on the webpage
    function updateMessages() {
      fetch('/messages')
        .then(response => response.json())
        .then(data => {
          const subscriptionMap = {
            'build_notifications_subscription': 'build_notifications_subscription',
            'deploy-commands-subscription': 'deploy_commands_subscription',
            'clouddeploy-operations-subscription': 'clouddeploy_operations_subscription',
            'clouddeploy-approvals-subscription': 'clouddeploy_approvals_subscription'
          };

          // Loop through each subscription and update its content
          for (const subscriptionName in subscriptionMap) {
            const messages = data[subscriptionName] || [];
            const messageContainer = document.getElementById(subscriptionMap[subscriptionName]);

            // Only update if there are new messages
            if (!lastMessageData[subscriptionName] || lastMessageData[subscriptionName].length !== messages.length) {
              messageContainer.innerHTML = ''; // Clear previous messages

              // Add new messages with pretty-printed JSON
              messages.forEach(message => {
                const messageDiv = document.createElement('div');
                messageDiv.className = 'message';
                messageDiv.textContent = JSON.stringify({
                  id: message.id,
                  data: message.data,
                  attributes: message.attributes
                }, null, 2); // Prettify JSON with indentation
                messageContainer.appendChild(messageDiv);
              });
            }
          }

          // Save the latest message data to compare on the next refresh
          lastMessageData = data;
        });
    }

    // Function to clear all messages (both frontend and backend)
    function clearMessages() {
      fetch('/clear-messages', {
        method: 'POST'
      })
      .then(response => {
        if (response.ok) {
          lastMessageData = {}; // Clear the frontend message data
          updateMessages(); // Update the frontend to show empty message boxes
        }
      })
      .catch(error => console.error('Error clearing messages:', error));
    }

    function updateMessages() {
    fetch('/messages')
      .then(response => response.json())
      .then(data => {
        const messageBox = document.getElementById('messageBox');
        messageBox.innerHTML = ''; // Clear previous messages

        for (const subscriptionName in data) {
          const messages = data[subscriptionName];
          messageBox.innerHTML += `<h2>${subscriptionName}:</h2>`;
          messages.forEach(message => {
            messageBox.innerHTML += `<p>${JSON.stringify(message, null, 2)}</p>`; // Pretty print JSON
          });
        }
      });
  }

  function sendMessage() {
    const messageInput = document.getElementById('message-input');
    const message = messageInput.value;

    // Send the message to the backend
    fetch('/send-message', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ message }),
    })
    .then(response => {
      if (response.ok) {
        messageInput.value = ''; // Clear the input after sending
        updateMessages(); // Optionally refresh messages after sending
      } else {
        console.error('Failed to send message');
      }
    })
    .catch(error => console.error('Error:', error));
  }

    // Automatically update messages every 3 seconds
    setInterval(updateMessages, 3000);
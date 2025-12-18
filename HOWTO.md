# Glooscap How-To Guide

This guide will walk you through using Glooscap to translate wiki pages after installation.

## Prerequisites

### Prerequisite 1: Installation Complete

You have successfully run `./install_glooscap.sh` on your machine and can access the UI at:

- **http://glooscap-ui.testdev.dasmlab.org:8080** (or **http://localhost:8080**)

> **Note:** The FQDN `glooscap-ui.testdev.dasmlab.org` is not an external domain. The installer automatically adds a host entry to your `/etc/hosts` file to make this work locally. If you prefer, you can use `http://localhost:8080` instead.

### Prerequisite 2: MCN VPN Connected and API Key Created

1. **Connect to MCN VPN**
2. **Log into the wiki.pes site**
3. **Click on your profile** (bottom right corner)
4. **Click on "API Key"**
5. **Create an API key for yourself** - you will need this in the next step

---

## Step 1: Configure the Wiki Target

1. **Navigate to Settings** → **WikiTarget Tab**
2. **Click "Add a new Target"**
3. **Fill in the form** with:
   - **Name**: Choose any name (e.g., "MCN Wiki")
   - **URL**: The URL of your wiki (e.g., `https://wiki.pes.example.com`)
   - **Secret**: (if required)
   - **API Token**: Paste the API key you created in Prerequisite 2
   - **Mode**: Select "ReadWrite" to enable translation
   - **Insecure Skip TLS Verify**: Enable if using self-signed certificates

> **Note:** A picture will be added here showing the form fields.

4. **Click "Save"** to create the WikiTarget

---

## Step 1b: Configure Translation Service

1. **Navigate to Settings** → **Translation Service Tab**
2. **Click the "Connect" button**
3. **Wait approximately 15 seconds**
4. **Verify that the connection status turns green** (indicating successful connection)

If the connection fails, check:
- The translation service is running (e.g., Iskoces is deployed)
- The service address is correct
- Network connectivity to the service

---

## Step 1c: Verify Operator Status

1. **Navigate to Settings** → **Main Tab**
2. **Verify that the Operator status is marked as green** (healthy)
   - Other components showing red is fine - only the Operator needs to be green for basic functionality

---

## Step 2: Refresh and View Catalogue

1. **Navigate to Catalogue**
2. **Click "Refresh Catalogue"**
3. **Observe that all pages within the MAURICE (PDG) collection are shown**

> **Note:** A picture will be added here showing the catalogue view with pages from the MAURICE (PDG) collection.

The catalogue will display:
- Page titles
- Last updated timestamps
- Translation status
- Collection information

---

## Step 3: Translate a Page (Author View)

1. **Navigate to Author View**
2. **Go to the page you want to translate**
3. **Select the source page** from the dropdown box
4. **Click "Translate"**
5. **Observe the page is loaded in the panel** with the translated content

The translation process will:
- Fetch the source page content
- Send it to the translation service
- Display the translated content in the preview panel

---

## Step 4: View Your Translated Draft

1. **Open the MCN Wiki** in your browser
2. **Click on "Drafts" (Brouillons)** in the navigation
3. **Find your translated page** - it will be prefixed with **"AUTOTRANSLATE"**

> **Note:** A picture will be added here showing the drafts folder with the AUTOTRANSLATE-prefixed page.

The translated page will appear as a draft in your wiki, ready for review and editing.

---

## Step 5: Final Edits and Publish

1. **Review the translated content** in the draft
2. **Make any necessary edits** to improve the translation or add context
3. **Choose your next action:**
   - **Share the draft** with team members for review
   - **Manually edit** the content to refine the translation
   - **Publish** the page to make it visible to others in your location

> **Note:** A picture will be added here showing the publish/share options.

Once published, the translated page will be available to all users in your wiki location.

---

## Troubleshooting

### WikiTarget Connection Issues

- Verify VPN is connected
- Check that the API token is correct and has proper permissions
- Ensure the wiki URL is accessible
- Check browser console for error messages

### Translation Service Connection Issues

- Verify the translation service (e.g., Iskoces) is running:
  ```bash
  kubectl get pods -n iskoces
  ```
- Check service logs:
  ```bash
  kubectl logs -n iskoces deployment/iskoces-server
  ```
- Verify the service address in Settings → Translation Service

### Catalogue Not Refreshing

- Ensure WikiTarget is configured and connected (green status)
- Check that the collection name matches (e.g., "Maurice (PDG)")
- Verify VPN connection is active
- Try clicking "Refresh Catalogue" again after a few seconds

### Translation Not Appearing in Drafts

- Check that WikiTarget mode is set to "ReadWrite"
- Verify translation job completed successfully (check logs)
- Ensure you're looking in the correct wiki and user account
- Check that the draft wasn't accidentally deleted

---

## Next Steps

- **Batch Translation**: Select multiple pages in the Catalogue and translate them all at once
- **Automated Translation**: Configure automatic translation for new pages
- **Translation Quality**: Review and refine translations for better accuracy
- **Team Collaboration**: Share drafts with team members for review before publishing

---

## Support

For issues or questions:
- Check the logs: `kubectl logs -n glooscap-system deployment/operator-controller-manager`
- Review the [README.md](README.md) for more information
- Check the [CHANGELOG.md](CHANGELOG.md) for known issues


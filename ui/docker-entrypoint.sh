#!/bin/sh
set -e

# Inject API_BASE_URL into index.html if provided
if [ -n "$API_BASE_URL" ]; then
  # Create a script tag that sets the API base URL
  SCRIPT_TAG="<script>window.__API_BASE_URL__='$API_BASE_URL';</script>"
  
  # Inject before closing </head> tag, or at the beginning of <body>
  if grep -q "</head>" /app/dist/spa/index.html; then
    sed -i "s|</head>|$SCRIPT_TAG</head>|" /app/dist/spa/index.html
  elif grep -q "<body>" /app/dist/spa/index.html; then
    sed -i "s|<body>|<body>$SCRIPT_TAG|" /app/dist/spa/index.html
  fi
  
  echo "Injected API_BASE_URL: $API_BASE_URL"
fi

# Serve static files with serve (handles SPA routing automatically)
exec serve -s /app/dist/spa -l 8080 --no-port-switching


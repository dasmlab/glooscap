# HAProxy CORS Configuration

If you have an HAProxy in front of your OpenShift Routes, you need to ensure it passes through CORS headers from the backend service.

## HAProxy Configuration

Add the following to your `backend ocp2backend` section to ensure CORS headers are passed through:

```haproxy
backend ocp2backend
  mode http
  
  # First, try to preserve CORS headers from backend (if backend responds)
  http-response set-header Access-Control-Allow-Origin %[res.hdr(Access-Control-Allow-Origin)] if { res.hdr(Access-Control-Allow-Origin) -m found }
  http-response set-header Access-Control-Allow-Credentials %[res.hdr(Access-Control-Allow-Credentials)] if { res.hdr(Access-Control-Allow-Credentials) -m found }
  http-response set-header Access-Control-Allow-Methods %[res.hdr(Access-Control-Allow-Methods)] if { res.hdr(Access-Control-Allow-Methods) -m found }
  http-response set-header Access-Control-Allow-Headers %[res.hdr(Access-Control-Allow-Headers)] if { res.hdr(Access-Control-Allow-Headers) -m found }
  http-response set-header Access-Control-Expose-Headers %[res.hdr(Access-Control-Expose-Headers)] if { res.hdr(Access-Control-Expose-Headers) -m found }
  
  # If backend didn't set CORS headers (e.g., 503 error), add them based on Origin header
  http-response set-header Access-Control-Allow-Origin %[req.hdr(Origin)] if { ! res.hdr(Access-Control-Allow-Origin) -m found } { hdr(Origin) -m found }
  http-response set-header Access-Control-Allow-Credentials "true" if { ! res.hdr(Access-Control-Allow-Credentials) -m found } { hdr(Origin) -m found }
  http-response set-header Access-Control-Allow-Methods "GET, POST, OPTIONS" if { ! res.hdr(Access-Control-Allow-Methods) -m found } { hdr(Origin) -m found }
  http-response set-header Access-Control-Allow-Headers "Content-Type, Authorization, Accept" if { ! res.hdr(Access-Control-Allow-Headers) -m found } { hdr(Origin) -m found }
  
  server router0 10.20.1.230:443 ssl sni req.hdr(Host) verify none check
```

**Important:** The order matters! First try to preserve backend headers, then add them if missing (for error responses like 503).

## OpenShift Router

The OpenShift Router uses HAProxy under the hood and **automatically passes through CORS headers** from the backend service. You don't need to configure anything on the Route itself - the backend (our operator) sets the CORS headers, and the router passes them through.

However, if you have an **external HAProxy** in front of the OpenShift Router, you need to configure it as shown above to preserve the CORS headers.

## Testing CORS

You can test if CORS headers are being passed through:

```bash
# Test from the UI origin
curl -H "Origin: https://web-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org" \
     -H "Access-Control-Request-Method: GET" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -X OPTIONS \
     -v \
     https://api-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org/api/v1/events

# Should return:
# Access-Control-Allow-Origin: https://web-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org
# Access-Control-Allow-Credentials: true
# Access-Control-Allow-Methods: GET, POST, OPTIONS
# Access-Control-Allow-Headers: Content-Type, Authorization, Accept
```

## Troubleshooting

If CORS headers are still missing:

1. **Check if HAProxy is stripping headers**: Look for `http-response del-header` or `http-request del-header` directives that might be removing CORS headers
2. **Check OpenShift Router logs**: `oc logs -n openshift-ingress-operator -l ingresscontroller.operator.openshift.io/deployment-ingresscontroller=default`
3. **Verify backend is setting headers**: Test directly against the service (bypassing the route): `curl -v http://operator-glooscap-operator-api.glooscap-system.svc.cluster.local:3000/api/v1/events`
4. **Check for multiple proxies**: If you have HAProxy → OpenShift Router → Service, ensure both are configured correctly


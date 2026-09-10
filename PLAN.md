I’d build lantern: a small private uptime monitor for services across your devices.
Run it on your laptop or a small VM. Give it a configuration like:
[[services]]
name = "podcast"
url = "http://podcast-server:8080/health"

[[services]]
name = "vm-broker"
url = "http://lab-server:8080/health"
It checks each service and shows a tiny dashboard: reachable, response time, last successful check. Useful for answering “is the machine offline, or is just the application broken?”
You’d exercise Tailscale through a few concrete steps:
1. Connect two devices and address them by name. Your monitor reaches services across networks without public application ports.
2. Publish the dashboard privately with Serve. This teaches private HTTPS and reverse proxying. Serve docs
3. Restrict access with grants. Give the monitor permission to reach only the service ports it checks. Existing broad grants must be removed or narrowed for that restriction to matter. Grants docs
4. Diagnose failures. Add lantern diagnose podcast to report DNS resolution, Tailscale connectivity, and the HTTP check separately. Inspect direct versus relayed connections using Tailscale’s diagnostic commands. CLI docs
5. Optional: expose a separate, sanitized status page through Funnel to learn the difference between tailnet-only and public access. Funnel docs
Keep v1 to one Go binary, a configuration file, an in-memory history, and one HTML page. No database, notifications, or cloud provisioning. Unit-test status transitions and timeouts; use a live check between two devices to verify the networking.
It complements your existing projects: cinders provides temporary compute, podcast-service produces something, vm-service manages machines, and lantern tells you whether their services are reachable.

# MASH TODO

Remaining work items for the MASH protocol and implementation.

## Testing Infrastructure

- [ ] Implement test harness to execute YAML test cases against devices
- [ ] Build PICS parser/validator tooling

## Features (Code Implementation)

- [ ] Implement Tariff feature
- [ ] Implement Signals feature
- [ ] Implement Plan feature

## Architecture

- [ ] Design ZONE_ADMIN_APP device type
  - Apps (mobile, voice assistants) are not zone types but devices within a zone
  - Apps can be authorized as zone admins to manage the zone (UI, QR scanner for commissioning)
  - Need to define: endpoint type, authorization flow, delegated permissions

---

*Last updated: 2025-01-27*

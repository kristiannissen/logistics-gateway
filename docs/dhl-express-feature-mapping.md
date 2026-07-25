# DHL Express — Feature Mapping

API: **MyDHL API v3.3.0**
Base URL (prod): `https://express.api.dhl.com/mydhlapi`
Auth: HTTP Basic (username + password)
Coverage: Worldwide — express international and domestic.
Implementation status: **Partial** — Cancel is a confirmed carrier limitation
(no void/cancel AWB endpoint in the MyDHL API). Update was previously
mis-credited as a full carrier limitation too, but the MyDHL API does expose
`PATCH /shipments/{id}/add-piece` (add a new package pre-pickup) — that was a
genuine implementation gap, now closed (see Update Shipment below). With that
fixed, all primary methods are complete. The remaining gap is secondary:
standalone pickup update/cancel exist in the MyDHL API (`PATCH /pickups`,
`DELETE /pickups/{id}`) but the adapter does not implement `ManifestAdapter`
at all — a genuine implementation gap (a previous version of this doc wrongly
claimed these were wired).

---

## Summary

DHL Express covers booking, tracking, label fetch, return labels, and — as of
this update — adding a new package to an already-booked shipment pre-pickup
(`add-piece`). Pickup is implicit in the booking call (a
`dispatchConfirmationNumber` is returned), but standalone pickup update/cancel
are not wired despite the MyDHL API supporting them. AWB cancellation is not
available via the MyDHL API — the shipment cannot be voided after booking; the
`DELETE /pickups/{id}` endpoint DHL exposes cancels the courier collection,
not the shipment itself. Manifest retrieval is available post-collection via
the image endpoint.

---

## Feature fit/gap

### Booking

| Feature | Implemented | Notes |
|---|---|---|
| Book shipment | ✅ | `POST /shipments` — label returned inline in booking response |
| Cancel shipment | ❌ | No void/cancel AWB endpoint in MyDHL API. Shipment cannot be voided after booking. `DELETE /pickups/{id}` exists but only cancels the courier collection, not the AWB. |
| Update shipment | ⚠️ | Partial — `PATCH /shipments/{id}/add-piece` adds a new package pre-pickup (`UpdateRequest.AddPiece`). No general field-level update endpoint exists; DHL recommends cancel-and-rebook for anything else. Gated behind DHL account enablement. |
| Idempotency key | ❌ | Client-side only |

### Labels

| Feature | Implemented | Notes |
|---|---|---|
| Print label | ✅ | `GET /shipments/{id}/get-image` |
| Label formats | ✅ | PDF, PNG, ZPL |
| Return label | ✅ | `DeliveryType=return` — uses `returnProductCode` (configurable via `DHL_EXPRESS_RETURN_PRODUCT_CODE`). Defaults to product code `P` (EXPRESS WORLDWIDE). |
| Manifest document | ✅ | `GET /shipments/{id}/get-image?typeCode=MANIFEST` — available post-collection |

### Tracking

| Feature | Implemented | Notes |
|---|---|---|
| Current status | ✅ | `GET /shipments/{id}/tracking` — normalized status |
| Event history | ✅ | Scan events returned in `events[]` |
| Estimated delivery | ✅ | Where returned by carrier |

### Pickup scheduling

| Feature | Implemented | Notes |
|---|---|---|
| Implicit pickup at booking | ✅ | `pickup.isRequested=true` in booking payload. Returns `dispatchConfirmationNumber`. |
| Book standalone pickup | ❌ | Not yet wired as `POST /api/pickups` |
| Update pickup | ❌ | `PATCH /pickups` exists in the MyDHL API but `DHLExpressAdapter` does not implement `ManifestAdapter` at all — genuine gap, not a limitation |
| Cancel pickup | ❌ | `DELETE /pickups/{dispatchConfirmationNumber}` exists in the MyDHL API but is not wired — genuine gap, not a limitation |

**Corrected note:** A previous version of this doc claimed pickup update and
cancel were "wired via the `ManifestAdapter` interface." That was wrong —
`internal/adapter/dhl_express.go` implements only `BookShipment`,
`TrackShipment`, `FetchLabel`, `CancelShipment`, and `UpdateShipment`; there is
no `BookPickup`/`UpdatePickup`/`CancelPickup`/`CloseManifest` on this adapter.
Pickup is currently only triggered implicitly via the booking call, and
standalone pickup management is unavailable through the gateway despite
existing in the MyDHL API.

### Manifest

| Feature | Implemented | Notes |
|---|---|---|
| Close manifest | ❌ | DHL Express does not use a pre-collection manifest close. The manifest document is retrieved post-collection. |
| Manifest document | ✅ | Available post-collection via `GET /shipments/{id}/get-image?typeCode=MANIFEST` |

### Add-ons

| Add-on | Implemented | Notes |
|---|---|---|
| SMS notification | ⚠️ | Accepted but triggers `AddOnWarning` — not supported by MyDHL API shipment endpoint |
| Email notification | ✅ | Mapped to `shipmentNotification` in payload. Requires `receiver.email`. |
| Flex delivery / on-demand | ✅ | `onDemandDelivery.servicePointId` for service point redirection |
| Signature required | ⚠️ | Accepted but triggers `AddOnWarning` — not wired |
| Cash on delivery | ⚠️ | Accepted but triggers `AddOnWarning` — not supported by booking endpoint |
| Insurance | ✅ | Mapped to `valueAddedServices` insurance block |

### Other features

| Feature | Implemented | Notes |
|---|---|---|
| Customs / cross-border | ✅ | Full customs declaration — `Customs` block with Incoterms, HS codes, EORI, IOSS, invoice number/date, line items. IOSS maps to `SDT` registration number on importer. |
| Service point delivery | ✅ | `receiver.servicePointId` → `onDemandDelivery.servicePointId` (6-char DHL code) |
| Multi-colli | ✅ | Multiple packages per shipment |
| Business delivery | ✅ | Product code selection |
| Domestic + international | ✅ | Product code `P` (EXPRESS WORLDWIDE) is the default; overridable |

---

## Endpoint mapping

| carrier-gateway | DHL Express API | Status |
|---|---|---|
| `POST /api/bookings` | `POST /shipments` | ✅ |
| `DELETE /api/bookings/{id}` | — | ❌ Not available → 501 |
| `PATCH /api/bookings/{id}` | `PATCH /shipments/{id}/add-piece` | ⚠️ Add-piece only (new package pre-pickup); other fields → 501 |
| `GET /api/trackings/{id}` | `GET /shipments/{id}/tracking` | ✅ |
| `GET /api/labels/{id}` | `GET /shipments/{id}/get-image` | ✅ |
| `POST /api/pickups` | Implicit via booking | ⚠️ Standalone not wired |
| `PUT /api/pickups/{id}` | `PATCH /pickups` (exists, unwired) | ❌ → 501 |
| `DELETE /api/pickups/{id}` | `DELETE /pickups/{id}` (exists, unwired) | ❌ → 501 |
| `POST /api/manifests` | `GET /shipments/{id}/get-image?typeCode=MANIFEST` | ✅ (post-collection only) |

---

## Environment variables

| Variable | Description |
|---|---|
| `DHL_EXPRESS_USERNAME` | MyDHL API username |
| `DHL_EXPRESS_PASSWORD` | MyDHL API password |
| `DHL_EXPRESS_ACCOUNT_NUMBER` | DHL Express account number |
| `DHL_EXPRESS_PRODUCT_CODE` | Product code for outbound shipments (e.g. `P`) |
| `DHL_EXPRESS_RETURN_PRODUCT_CODE` | Product code for return shipments |

---

## Implementation notes

**No AWB cancellation.** DHL Express does not expose a cancel/void shipment
endpoint. Once a shipment is booked, it cannot be cancelled via API. `CancelShipment`
returns `ErrNotSupported`. The pickup can still be cancelled independently via
`DELETE /pickups/{dispatchConfirmationNumber}`.

**Pickup confirmation number.** The `dispatchConfirmationNumber` from
`BookingResponse` is required to update or cancel the pickup. It is separate
from the AWB tracking number. Callers must store it at booking time.

**Product code.** Defaults to `P` (EXPRESS WORLDWIDE). Override via the
`DHL_EXPRESS_PRODUCT_CODE` environment variable. Return shipments use
`DHL_EXPRESS_RETURN_PRODUCT_CODE`.

**Add-piece update (`UpdateShipment`).** `PATCH /shipments/{id}/add-piece` is
the only shipment-update capability the MyDHL API exposes, and only before
the shipment has been picked up. Set `UpdateRequest.AddPiece` (weight,
dimensions, reference, description); every other `UpdateRequest` field
(contact details, weight-on-existing-package, service point) returns
`ErrNotSupported` — DHL recommends cancelling and rebooking for anything
beyond adding a piece. Two known limitations, both a consequence of the
gateway being stateless:
- **`originalPlannedShippingDate`** must match the date from the original
  `BookShipment` call. The gateway does not persist it, so pass it back via
  `AddPiece.OriginalPlannedShippingDate`; if omitted, the adapter falls back
  to today's date on a best-effort basis, which DHL Express may reject.
- **`productCode`** is not tracked from the original booking either, so the
  request falls back to the account's default product code
  (`DHL_EXPRESS_PRODUCT_CODE`), which may not match a return shipment booked
  with `DHL_EXPRESS_RETURN_PRODUCT_CODE`.

Access to `add-piece` is gated per DHL account — a 403 means it needs to be
enabled by your DHL Express representative, not a bug in the adapter.

The MyDHL API also documents `PATCH /shipments/{id}/upload-invoice-data`
(revise customs invoice data pre-pickup). It is not wired — only `add-piece`
has been confirmed against DHL's sandbox and implemented. Worth a follow-up
once there's a concrete need to update customs data post-booking.

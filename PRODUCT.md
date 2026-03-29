# BitRiver Studio OS Product Definition

## Vision

BitRiver Studio OS turns a small x86_64 PC into a dedicated live production appliance.

The product boots directly into BitRiver, is controlled entirely from a browser on the local network, and replaces the fragile "general-purpose PC plus a pile of apps" setup with a focused, self-hosted broadcast system.

The MVP promise is simple:

- Install on a mini PC.
- Power it on.
- Claim it from a browser on the LAN.
- Add cameras, audio, and network contributors.
- Go live and record reliably without babysitting a desktop operating system.

This is an appliance product, not a Linux distro for creators. Every major decision should favor operational reliability, fast recovery, and a crisp first-run experience over flexibility for power users.

## Product Principles

- Appliance first. BitRiver should feel like a switcher or encoder appliance, not like a computer.
- Browser first. All operator workflows must be available from the LAN web UI without SSH, a local desktop, or native apps.
- Proven media stack. MVP stands on OBS, MediaMTX, PipeWire, and Owncast instead of inventing new media primitives.
- Safe by default. Updates, reboot behavior, rollback, and reset flows must be explicit and boring.
- Self-hosted by default. No cloud account is required to operate the box.

## Problem

Independent streamers and small production teams often assemble live workflows from a gaming PC, OBS profiles, USB capture gear, browser tabs, and ad hoc ingest tools. The result is powerful but fragile:

- OS updates land at the wrong time.
- Scene collections drift.
- Audio routing becomes opaque.
- Remote operation requires awkward remote desktop sessions.
- Recovery after crashes or bad config changes is stressful.

BitRiver Studio OS exists to package a stable subset of that stack into a bootable production appliance with a strong operational model.

## Target Users

### 1. Independent Streamer

A solo creator with one or two cameras, a USB mic or audio interface, and one gameplay or laptop capture input.

What they need:

- A box that is more reliable than a reused gaming PC.
- Easy remote control from a laptop, tablet, or phone on the same network.
- Local recording plus one live destination.
- Confidence that a reboot or update will not destroy the setup.

### 2. Small Show Operator

A two- or three-person team producing podcasts, interviews, or creator collaborations from a small studio.

What they need:

- Scene switching from a laptop on the LAN.
- A clean way to bring in a remote guest or phone feed.
- Predictable audio routing.
- Clear health indicators during a show.

### 3. Self-Hosting Broadcaster

A creator who wants to own their distribution path and optionally stream to a bundled local Owncast instance instead of depending only on a third-party platform.

What they need:

- An appliance that can run Owncast locally.
- A browser-managed control surface.
- A path to self-hosted streaming without building a rack of services.

## MVP Product Statement

BitRiver Studio OS MVP is a single-box live production appliance for x86_64 mini PCs that:

- boots into a locked-down BitRiver control plane
- manages capture, scenes, streaming, and recording through OBS
- accepts network contributor feeds through MediaMTX
- uses PipeWire for local audio and video device orchestration where appropriate
- optionally runs Owncast as a local live destination
- is fully operable from a browser on the LAN
- supports safe updates, rollback, crash recovery, and factory reset

## MVP Scope

### In Scope

- Bootable immutable Linux image for x86_64 mini PCs.
- Browser-based control plane served from the appliance.
- Local HDMI display boots into the same control plane in kiosk mode.
- First-boot claim flow and admin password setup.
- Device discovery for supported local capture and audio hardware.
- OBS scene switching, start/stop streaming, start/stop recording, mute/unmute, gain adjustment, and source visibility control through `obs-websocket`.
- Program recording to MKV with optional post-show remux to MP4.
- MediaMTX-managed contributor ingest for RTMP and SRT inputs.
- Fixed ingest slots with copyable publish URLs and status.
- Optional Owncast bundle as a selectable live destination.
- Browser dashboard for health, active scene, audio levels, disk space, destination status, and recent events.
- Safe updates with staged apply and rollback.
- Factory reset that clears operational state and secrets.

### MVP Source Types

- USB UVC cameras.
- HDMI capture devices that present as UVC/UAC devices.
- USB audio interfaces and microphones supported by PipeWire and ALSA.
- Network contributor inputs published to MediaMTX over RTMP or SRT.
- Browser sources and overlays configured inside OBS from approved URLs.

### MVP Output Modes

- One active live destination at a time.
- Simultaneous local program recording.
- Supported live destinations are an external RTMP platform such as Twitch or YouTube, or the bundled local Owncast server.

### MVP Operator Workflows

- Claim a new appliance from a browser on the LAN.
- Verify hardware and source readiness.
- Configure a show with scenes, sources, and one live destination.
- Generate guest ingest URLs for remote contributors.
- Start recording before going live.
- Switch scenes and monitor health during the show.
- Stop stream and recording, then export or retrieve recordings over the LAN.
- Install an update after the show and reboot safely.
- Reset the box to factory state when repurposing or reselling it.

## Definition of a Polished MVP

The MVP is successful if a streamer can unbox a supported mini PC, install BitRiver Studio OS, and run a real show without opening a Linux shell or touching a desktop environment.

Concrete acceptance criteria:

- First boot to controllable UI in under 90 seconds on target hardware.
- Remote browser onboarding in under 5 minutes.
- Stable 1080p program streaming and recording on supported hardware.
- Program recording survives application crashes by using MKV.
- A failed update does not strand the device in an unusable state.
- A bad configuration apply can roll back to the last known-good generation.

## Feature Pillars

### 1. Dedicated Broadcast Appliance

BitRiver should behave like a piece of production gear. It powers on into the live environment, exposes only the controls that matter, and avoids desktop distractions.

### 2. Browser-Native Operations

The control plane must be good enough that an operator prefers it to remote desktop. Scene switching, audio control, ingest management, destination management, and update operations all happen in the browser.

### 3. Reliable Live Show Core

OBS handles composition, scene logic, streaming, and program recording. MediaMTX handles network ingest and path routing. PipeWire provides a modern local graph for audio and device handling. BitRiver's job is to make those components behave like a cohesive appliance.

### 4. Self-Hosted Ownership

The appliance must be useful without cloud accounts, and Owncast must be a first-class optional path for creators who want a self-hosted destination.

## Practical Product Choices

These choices optimize for the fastest path to a credible, impressive MVP:

- Single appliance box first. No multi-node control room design in MVP.
- Wired LAN first. Wi-Fi management is not part of the initial promise.
- One live destination at a time. Multi-destination streaming is deferred.
- Supported hardware matrix instead of "works with everything."
- Fixed ingest slots for guests instead of a fully generic routing studio.
- No plugin marketplace. The system ships a curated stack only.
- No remote internet exposure feature in MVP. The appliance is designed for trusted LAN operation.

## Non-Goals

- General desktop computing.
- Arbitrary package installation on the appliance.
- OBS plugin marketplace or end-user plugin installs.
- Full NDI workflow support in MVP.
- Blackmagic DeckLink support in MVP.
- Multi-box clustering, failover pairs, or distributed switching.
- Cloud accounts, cloud sync, or SaaS dependencies.
- Built-in remote access over the public internet.
- Advanced graphics authoring, titling, or editing tools.
- AI scene automation, transcript generation, or highlight clipping in MVP.

## Risks and Mitigations

### OBS Runtime Reliability on Immutable Linux

Risk:
OBS is a desktop-originated application, and reliable appliance operation on Linux requires careful session management.

Mitigation:
Use a locked-down compositor session, keep the plugin set minimal, require supported GPU paths, and treat OBS as a supervised service with health checks and restart rules.

### Hardware Compatibility Drift

Risk:
Capture cards, webcams, audio interfaces, and mini PC firmware vary widely.

Mitigation:
Publish a narrow compatibility matrix for MVP, target UVC/UAC first, and certify a small set of mini PCs and capture devices before claiming broad support.

### Audio Complexity

Risk:
Audio routing is often the least forgiving part of live production.

Mitigation:
Expose only clear mixer operations in MVP, use PipeWire with predictable device naming, and avoid advanced patch-bay UX in the first release.

### Operator Expectations for Remote Preview

Risk:
Operators may expect a full browser-based multiview immediately.

Mitigation:
MVP focuses on control reliability, scene confidence thumbnails, meters, and health first. Full low-latency browser multiview is a roadmap item unless it proves stable enough during implementation.

### Update Anxiety

Risk:
Creators will not trust appliance updates unless rollback is obvious and dependable.

Mitigation:
Use staged immutable updates, candidate boot validation, automatic rollback on failed health checks, and clear UI language around "install later" versus "reboot now."

## Roadmap

### Phase 1: MVP

- Immutable x86_64 appliance image.
- Browser-first control plane.
- OBS-driven switching, streaming, and MKV recording.
- MediaMTX RTMP and SRT contributor ingest.
- PipeWire-based local device handling.
- Optional Owncast destination.
- Safe updates, rollback, and factory reset.

### Phase 2: Broadcast Confidence

- Better browser-based preview and multiview.
- Certified hardware list with automated validation checks.
- More flexible scene templates and source slotting.
- Better recording management and clip export.
- Smarter network diagnostics and stream health warnings.

### Phase 3: Growth

- Multi-destination streaming.
- Remote guest workflows with stronger onboarding.
- More advanced automation and show presets.
- Optional remote administration patterns built on secure overlays or VPN guidance.

## Demo Scenario

This is the demo that should make the project feel real on GitHub:

1. Flash BitRiver Studio OS to a supported mini PC and boot it on a studio desk.
2. The local HDMI output shows a clean BitRiver setup screen with `bitriver.local` and a QR code.
3. From a laptop on the same LAN, the operator claims the device, sets an admin password, and lands in the control plane.
4. The dashboard shows one USB camera, one HDMI capture card, one USB mic, and available disk space.
5. The operator copies a guest ingest URL for `guest-1` and publishes to it from a phone or remote laptop using SRT or RTMP.
6. The guest feed appears as a healthy source, and the operator switches between `starting-soon`, `host-cam`, `split-guest`, and `full-screen-capture` scenes.
7. The operator enables local recording, selects Twitch or local Owncast as the destination, and starts the stream from the browser.
8. During the show, the dashboard displays dropped-frame warnings, audio levels, bitrate, source status, and remaining storage.
9. After the show, the operator stops the stream, keeps the recording, and schedules an update for later that night.
10. On reboot, the update either comes up healthy or the box automatically rolls back to the previous known-good version.

If this experience feels smooth, focused, and trustworthy, BitRiver Studio OS has a compelling MVP story.

# Third-party notices

Enhanced PS5 DualSense Bridge
Integration author: Ragnarok179

## SAxense
https://github.com/egormanga/SAxense
License: Mozilla Public License 2.0.
Public DualSense wireless audio-haptics research was used as a protocol reference for the signed stereo haptic payload and packet timing. No SAxense source is redistributed.

## Gamepad-Core / Dualsense-Multiplatform
https://github.com/rafaelvaloto/Dualsense-Multiplatform
License: MIT, Copyright (c) 2026 rafaelvaloto.
Used as a reference for DualSense state handling and output-report construction. No source file from the project is redistributed.

## Gamepad-Core-Tests
https://github.com/rafaelvaloto/Gamepad-Core-Tests
License: MIT, Copyright (c) 2026 rafaelvaloto.
Commit reviewed: 7a536ee95dcb49755f83767dcdc189bdf76a4fdd.
Used as a Windows output-report behavior reference. No source file from the project is redistributed.

## SDL
https://github.com/libsdl-org/SDL
License: zlib.
Used as a protocol reference for coalesced DualSense output and independent trigger validity bits. No SDL source or binary is redistributed.

## Linux hid-playstation
https://github.com/torvalds/linux/blob/master/drivers/hid/hid-playstation.c
License: GPL-2.0-or-later reference only.
Used to verify DualSense output validity flags and lightbar behavior. No Linux kernel source is redistributed.

## DS5Dongle
https://github.com/awalol/DS5Dongle
Protocol reference only. The referenced `audio.cpp` source declares Mozilla Public License 2.0.
Used to verify the Bluetooth report `0x36` layout, SetStateData block, haptic payload, counters and wireless state. No DS5Dongle source is redistributed.

## BeamNG.drive FFmpeg runtime interoperability

The Bluetooth controller-speaker path can dynamically load the FFmpeg libraries already installed by the user's own BeamNG.drive copy (`Bin64/VideoStream/avcodec-62.dll` and `avutil-60.dll`) to encode locally decoded BeamNG collision PCM as Opus.

The project does **not** redistribute BeamNG audio assets or BeamNG's FFmpeg binaries. Those files remain part of the user's local BeamNG installation.

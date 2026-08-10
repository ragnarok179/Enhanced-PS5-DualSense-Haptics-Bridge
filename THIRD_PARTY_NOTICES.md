Enhanced PS5 DualSense Haptics
Integration author: Ragnarok179

SAxense
https://github.com/egormanga/SAxense
License: Mozilla Public License 2.0.
The Bluetooth haptics-only report 0x32, 3 kHz signed stereo layout and packet
timing are based on SAxense's public research.

Gamepad-Core / Dualsense-Multiplatform
https://github.com/rafaelvaloto/Dualsense-Multiplatform
License: MIT, Copyright (c) 2026 rafaelvaloto.
Used for the DualSense state model and output report construction.

Gamepad-Core-Tests — Windows platform policy
https://github.com/rafaelvaloto/Gamepad-Core-Tests
License: MIT, Copyright (c) 2026 rafaelvaloto.
Commit reviewed: 7a536ee95dcb49755f83767dcdc189bdf76a4fdd.
The Windows policy is the reference for exact WriteFile lengths: 78 bytes for
0x31, 142 bytes for 0x32 and 398 bytes for advanced haptic reports.

SDL
https://github.com/libsdl-org/SDL
License: zlib.
Used as a reference for coalesced DualSense output and independent trigger
validity bits.

Linux hid-playstation
https://github.com/torvalds/linux/blob/master/drivers/hid/hid-playstation.c
GPL-2.0-or-later reference only. Used to verify that valid_flag0/1/2 select
independent output groups and that the lightbar uses valid_flag1 bit 2. No
kernel source is redistributed.

DS5Dongle
https://github.com/awalol/DS5Dongle
Protocol reference only. Current audio.cpp source declares Mozilla Public
License 2.0. Used to verify the 398-byte report 0x36 layout, SetStateData block,
64-byte haptic block, packet counters and persistent FD/F7 wireless state.
No DS5Dongle source file is redistributed in this package.

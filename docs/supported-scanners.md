# Supported Scanners 🖨️

UniverScan is designed to be **truly universal**. Instead of locking you into one manufacturer's proprietary software, UniverScan speaks the open standard languages that almost all modern and classic network scanners use.

---

## Brand Compatibility

| Brand | Compatibility Level | Notes |
| :--- | :--- | :--- |
| **Samsung** | 🟢 **100% Native** | Direct support for Xpress M2070, M2880, M2075, C480, and all network MFPs |
| **HP (Hewlett-Packard)** | 🟢 **Universal** | LaserJet, OfficeJet, PageWide, Envy, and DeskJet with network scanning |
| **Canon** | 🟢 **Universal** | imageCLASS, MAXIFY, PIXMA, and i-SENSYS series |
| **Epson** | 🟢 **Universal** | EcoTank, WorkForce, Expression, and Stylus network MFPs |
| **Brother** | 🟢 **Universal** | MFC, DCP, and HL series multi-function centers |
| **Xerox** | 🟢 **Universal** | WorkCentre, Phaser, VersaLink, and B-series multi-function printers |
| **Lexmark** | 🟢 **Universal** | MC, MB, CX, and MX series multi-function devices |
| **Kyocera / Ricoh / Sharp** | 🟢 **Universal** | TASKalfa, ECOSYS, IM, and Aficio office scanners |

---

## How UniverScan Talks to Your Scanner

You don't need to understand these technologies to use UniverScan, but here is what makes it work under the hood:

1. **AirScan / eSCL / Mopria:** The standard scanner protocol supported by Apple, Microsoft, Android, and all major printer manufacturers since 2013.
2. **WS-Scan (Web Services for Devices):** The native Microsoft Windows scanning protocol built into enterprise and home scanners.
3. **Samsung / Xerox Direct Protocol:** Raw binary TCP communication (Port 9400) for legacy and classic multifunction printers.

---

## Doesn't See Your Scanner?

If your scanner is connected to your Wi-Fi or network cable but UniverScan doesn't see it immediately:

1. Click the scanner name at the top of the UniverScan window.
2. Type in your scanner's IP address (for example, `192.168.1.50`). You can find this address on your printer's small screen under *Network Information* or *Wi-Fi Setup*.
3. Click **Connect**. UniverScan will connect to it and remember it for next time!

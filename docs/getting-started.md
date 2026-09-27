# Getting Started with UniverScan 🚀

UniverScan is designed so that anyone can download it and start scanning in less than a minute. You do not need to install any printer driver packages or create any online accounts.

---

## Windows (11, 10, 8, & Windows 7)

<p style="display: flex; align-items: center; gap: 8px; font-weight: 600; margin-top: 0.5rem;">
  <img src="assets/icons/windows.svg" width="20" height="20" alt="Windows">
  Supports 64-bit and 32-bit Windows PCs (Windows 11, 10, 8, and Windows 7 SP1)
</p>

1. **Download the Installer**  
   Download the [**UniverScan-Setup.exe**](https://github.com/arnaudbletterer/universcan/releases/latest/download/UniverScan-Setup.exe) installer.

2. **Install in One Click**  
   Double-click the installer, then click **Next** and **Install**. You do **not** need an Administrator password; it installs directly into your user apps folder.

3. **Start Scanning**  
   UniverScan will open automatically, and you will have a handy icon right on your Desktop ready to use.

> **Portable Option (No Installation):**  
> If you cannot install software on your computer (for example at school or on a restricted work computer), download [**universcan-windows-x86_64.zip**](https://github.com/arnaudbletterer/universcan/releases/latest) (or `universcan-windows-i386.zip` for 32-bit PCs). Extract it and run `universcan.exe` directly!

---

## Apple Mac (macOS)

<p style="display: flex; align-items: center; gap: 8px; font-weight: 600; margin-top: 0.5rem;">
  <img src="assets/icons/apple.svg" width="20" height="20" alt="Apple macOS">
  Supports Apple Silicon (M1, M2, M3, M4) and Intel Macs
</p>

1. **Download the Mac Package**  
   Download the [**UniverScan-macOS.pkg**](https://github.com/arnaudbletterer/universcan/releases/latest/download/UniverScan-macOS.pkg) 1-click installer, or the [**UniverScan-macOS.dmg**](https://github.com/arnaudbletterer/universcan/releases/latest/download/UniverScan-macOS.dmg) disk image.

2. **Install in Your Applications Folder**  
   Choose your preferred installer:
    - **Using `.pkg` (Recommended):** Double-click `UniverScan-macOS.pkg` and follow the quick wizard (*Continue → Install*). It installs automatically into `/Applications`.
    - **Using `.dmg`:** Double-click `UniverScan-macOS.dmg` and drag the **UniverScan** icon into the **Applications** folder.

3. **Launch UniverScan**  
   Open UniverScan from your **Applications** folder or Launchpad.

---

## Linux (Ubuntu, Debian, Fedora, Arch, Raspberry Pi)

<p style="display: flex; align-items: center; gap: 8px; font-weight: 600; margin-top: 0.5rem;">
  <img src="assets/icons/linux.svg" width="20" height="20" alt="Linux">
  Self-contained static executable with zero external library locks
</p>

1. **Download the Package for Your System**  
   Select the download for your computer architecture:
    - [**universcan-linux-x86_64.tar.gz**](https://github.com/arnaudbletterer/universcan/releases/latest) for standard 64-bit PCs
    - [**universcan-linux-i386.tar.gz**](https://github.com/arnaudbletterer/universcan/releases/latest) for older 32-bit computers
    - [**universcan-linux-arm64.tar.gz**](https://github.com/arnaudbletterer/universcan/releases/latest) for Raspberry Pi 3/4/5

2. **Extract and Run**  
   Open your terminal in your download folder and run:

        tar -xvf universcan-linux-x86_64.tar.gz
        ./universcan-linux-amd64

3. **Start Scanning**  
   UniverScan starts instantly and opens your browser window.

---

## Connecting Your Scanner

When you open UniverScan for the first time:

1. Make sure your scanner is turned on and connected to the same Wi-Fi or home network as your computer.
2. UniverScan will automatically look for your scanner.
3. Once found, you will see a green light: **Connected (Ready)**.
4. If your scanner is not detected automatically, click the scanner button at the top, type your scanner's IP address (e.g. `192.168.1.50`), and click **Connect**.

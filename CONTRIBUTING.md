# Contributing to UniverScan

Welcome! We are delighted that you want to help make document scanning simple, calm, and accessible to everyone around the world — from curious kids to grandparents.

Whether you want to report a scanner model, translate the app into your language, fix a bug, or improve documentation, your help is warmly appreciated!

---

## Ways to Contribute

### 1. Help Us Support More Scanners 🖨️
You don't need to write code to help UniverScan support every scanner on the planet:
1. Open UniverScan and connect your scanner.
2. Click the **Help** button in the top corner.
3. Click **Test Scanner Connection**, then click **Copy Help Information**.
4. Open an issue on GitHub using our [Scanner Report Template](.github/ISSUE_TEMPLATE/new_scanner_support.md) and paste what you copied!

Our team and automated agents use this exact report to ensure UniverScan connects to your scanner model seamlessly.

### 2. Add New Languages & Translations 🌍
We want UniverScan to be usable by people everywhere in their native language:
- All user-facing strings are cleanly located in `web/index.html` and `web/js/app.js`.
- If you would like to help translate UniverScan into your language, open an issue or pull request!

### 3. Improve Accessibility & Usability ♿
We believe software should be:
- Obvious to use without instructions.
- Senior-friendly with high-contrast text and tactile, forgiving touch/click targets.
- Calm, with clear non-alarming notifications when paper trays are empty or devices are asleep.

If you have feedback on improving usability for children, seniors, or people using screen readers, we would love to hear it.

### 4. Code & Technical Contributions 💻
UniverScan is written in standard modern Go with pure offline vanilla HTML/CSS/JavaScript (no bulky electron, no external runtime dependencies).

#### Local Development Setup
- Install [Go 1.22+](https://go.dev/dl/).
- Clone the repository:
  ```bash
  git clone https://github.com/abletterer/universcan.git
  cd universcan
  ```
- Run the test suite:
  ```bash
  go test ./...
  ```
- Run the local application:
  ```bash
  go run ./cmd/universcan
  ```
- UniverScan will start locally at `http://127.0.0.1:8765` and automatically open in your default browser.

#### Building All Platform Packages
```powershell
# On Windows (PowerShell):
.\scripts\build_all.ps1

# On Linux or macOS (Bash):
./scripts/build_all.sh
```

---

## Submitting Pull Requests

1. Fork the repository and create your branch from `main`:
   ```bash
   git checkout -b feature/my-friendly-improvement
   ```
2. Keep changes simple and surgical: touch only what is necessary.
3. Verify that all automated tests pass:
   ```bash
   go test -v ./...
   ```
4. Commit your changes with clear, friendly messages.
5. Open a Pull Request on GitHub. We review and merge community contributions quickly!

---

## Community Guidelines

Please adhere to our [Code of Conduct](CODE_OF_CONDUCT.md) in all project discussions, issues, and code reviews. Thank you for making technology kinder and simpler for everyone!

import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @AppStorage("binaryPath") private var binaryPath = "/opt/homebrew/bin/cc-journal"
    @AppStorage("journalDirectory") private var journalDirectory = "~/claude-journal"
    @AppStorage("launchAtLogin") private var launchAtLogin = false
    @AppStorage("serverPort") private var serverPort = 8000
    @AppStorage("autoStartServer") private var autoStartServer = false

    var body: some View {
        Form {
            Section("General") {
                TextField("cc-journal binary path", text: $binaryPath)
                TextField("Journal directory", text: $journalDirectory)
                Toggle("Launch at login", isOn: $launchAtLogin)
            }

            Section("Server") {
                TextField("Port", value: $serverPort, format: .number)
                    .onChange(of: serverPort) { _, newValue in
                        appState.serverManager.port = newValue
                    }
                Toggle("Start server on app launch", isOn: $autoStartServer)
            }

            Section("Keyboard Shortcut") {
                Text("Toggle popover: \u{2318}\u{21E7}J")
                    .foregroundStyle(.secondary)
            }
        }
        .formStyle(.grouped)
        .frame(width: 420, height: 300)
    }
}

import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @AppStorage("binaryPath") private var binaryPath = ""
    @AppStorage("launchAtLogin") private var launchAtLogin = false
    @AppStorage("serverPort") private var serverPort = 8000
    @AppStorage("autoStartServer") private var autoStartServer = false

    @State private var portText = ""

    var body: some View {
        Form {
            Section("General") {
                HStack {
                    TextField("cc-journal binary", text: $binaryPath, prompt: Text("Auto-detect"))
                    Button("Browse...") {
                        let panel = NSOpenPanel()
                        panel.canChooseFiles = true
                        panel.canChooseDirectories = false
                        panel.allowsMultipleSelection = false
                        panel.message = "Select the cc-journal binary"
                        if panel.runModal() == .OK, let url = panel.url {
                            binaryPath = url.path
                        }
                    }
                }
                Text("Leave empty to auto-detect. Requires app restart to take effect.")
                    .font(.caption)
                    .foregroundStyle(.secondary)

                Toggle("Launch at login", isOn: $launchAtLogin)
            }

            Section("Server") {
                TextField("Port", text: $portText)
                    .onAppear { portText = "\(serverPort)" }
                    .onSubmit {
                        if let port = Int(portText), port > 0, port <= 65535 {
                            serverPort = port
                            appState.serverManager.port = port
                        } else {
                            portText = "\(serverPort)"
                        }
                    }
                Toggle("Start server on app launch", isOn: $autoStartServer)
            }

            Section {
                LabeledContent("Binary") {
                    Text(CLIBridge.resolvedBinaryPath)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                }
                LabeledContent("Version") {
                    Text(appState.cliVersion.isEmpty ? "—" : appState.cliVersion)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                LabeledContent("Config") {
                    Text(Self.configFilePath)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                }
            } header: {
                Text("Detected")
            }
        }
        .formStyle(.grouped)
        .frame(width: 520, height: 500)
        .frame(minWidth: 400, minHeight: 300)
    }

    private static var configFilePath: String {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        let path = "\(home)/.config/cc-journal/config.yaml"
        if FileManager.default.fileExists(atPath: path) {
            return path
        }
        return "\(path) (not found)"
    }
}

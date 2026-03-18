import SwiftUI

@main
struct CCJournalApp: App {
    @State private var appState = AppState()

    var body: some Scene {
        MenuBarExtra("cc-journal", systemImage: "book.closed.fill") {
            PopoverView()
                .environment(appState)
        }
        .menuBarExtraStyle(.window)

        Settings {
            SettingsView()
                .environment(appState)
        }
    }
}

@Observable
final class AppState {
    var stats: StatsResponse?
    var todayEntries: [JournalEntry] = []
    var isLoading = false
    var errorMessage: String?
    var serverManager: ServerManager

    private let cli = CLIBridge()
    private var fileWatcher: FileWatcher?

    init() {
        let binaryPath = CLIBridge.resolvedBinaryPath
        serverManager = ServerManager(
            binaryPath: binaryPath,
            port: UserDefaults.standard.object(forKey: "serverPort") as? Int ?? 8000
        )
        startWatching()
        autoStartServerIfEnabled()
    }

    func refresh() async {
        isLoading = true
        errorMessage = nil

        async let statsResult = cli.fetchStats()
        async let todayResult = cli.fetchToday()

        do {
            stats = try await statsResult
        } catch {
            errorMessage = "Failed to load stats: \(error.localizedDescription)"
        }

        do {
            let response = try await todayResult
            todayEntries = response.entries
        } catch {
            if errorMessage == nil {
                errorMessage = "Failed to load entries: \(error.localizedDescription)"
            }
        }

        isLoading = false
    }

    private func startWatching() {
        fileWatcher = FileWatcher(directory: Self.journalDirectory) { [weak self] in
            guard let self else { return }
            Task { @MainActor in
                await self.refresh()
            }
        }
        fileWatcher?.start()
    }

    private func autoStartServerIfEnabled() {
        if UserDefaults.standard.bool(forKey: "autoStartServer") {
            serverManager.startServer()
        }
    }

    static var journalDirectory: String {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        return "\(home)/claude-journal"
    }
}

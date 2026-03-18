import AppKit
import os.log
import SwiftUI

private let logger = Logger(subsystem: "com.ccjournal.app", category: "App")

@main
struct CCJournalApp: App {
    @State private var appState = AppState()

    var body: some Scene {
        MenuBarExtra("cc-journal", systemImage: "book.closed.fill") {
            PopoverView()
                .environment(appState)
        }
        .menuBarExtraStyle(.window)
    }
}

/// Manages a standalone NSWindow for settings.
/// Menu bar-only apps need explicit activation policy to show windows.
final class SettingsWindowController {
    static let shared = SettingsWindowController()
    private var window: NSWindow?
    private var hostingView: NSHostingView<AnyView>?

    func show(appState: AppState) {
        if let window, window.isVisible {
            window.makeKeyAndOrderFront(nil)
            NSApp.setActivationPolicy(.accessory)
            NSApp.activate(ignoringOtherApps: true)
            return
        }

        let settingsView = AnyView(
            SettingsView()
                .environment(appState)
        )

        let hosting = NSHostingView(rootView: settingsView)
        hosting.frame = NSRect(x: 0, y: 0, width: 520, height: 500)
        self.hostingView = hosting

        let window = NSWindow(
            contentRect: hosting.frame,
            styleMask: [.titled, .closable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.minSize = NSSize(width: 400, height: 300)
        window.title = "cc-journal Settings"
        window.contentView = hosting
        window.center()
        window.isReleasedWhenClosed = false
        window.level = .floating

        self.window = window

        // Ensure the app can present windows
        NSApp.setActivationPolicy(.accessory)
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

@Observable
final class AppState {
    var stats: StatsResponse?
    var todayEntries: [JournalEntry] = []
    var isLoading = false
    var errorMessage: String?
    var cliVersion: String = ""
    var serverManager: ServerManager

    private let cli = CLIBridge()
    private var fileWatcher: FileWatcher?

    init() {
        logger.info("AppState initializing")
        let binaryPath = CLIBridge.resolvedBinaryPath
        serverManager = ServerManager(
            binaryPath: binaryPath,
            port: UserDefaults.standard.object(forKey: "serverPort") as? Int ?? 8000
        )
        startWatching()
        autoStartServerIfEnabled()
        logger.info("AppState ready")
    }

    func refresh() async {
        logger.debug("Refreshing data")
        isLoading = true
        errorMessage = nil

        // Fetch version on first load or if empty
        if cliVersion.isEmpty {
            do {
                cliVersion = try await cli.fetchVersion()
                logger.info("CLI version: \(self.cliVersion)")
            } catch {
                logger.warning("Could not fetch CLI version: \(error.localizedDescription)")
            }
        }

        async let statsResult = cli.fetchStats()
        async let todayResult = cli.fetchToday()

        do {
            stats = try await statsResult
        } catch {
            logger.error("Failed to load stats: \(error.localizedDescription)")
            errorMessage = "Failed to load stats: \(error.localizedDescription)"
        }

        do {
            let response = try await todayResult
            todayEntries = response.entries
            logger.info("Loaded \(response.entries.count) entries for today")
        } catch {
            logger.error("Failed to load entries: \(error.localizedDescription)")
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

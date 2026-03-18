import SwiftUI

struct PopoverView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("cc-journal")
                    .font(.headline)
                Spacer()
                ServerStatusBadge(state: appState.serverManager.state, port: appState.serverManager.port)
                SettingsLink {
                    Image(systemName: "gearshape")
                }
                .buttonStyle(.plain)
            }
            .padding(.horizontal)
            .padding(.top, 12)
            .padding(.bottom, 8)

            Divider()

            ScrollView {
                VStack(spacing: 16) {
                    if let stats = appState.stats {
                        StatsView(stats: stats)
                    }

                    EntryListView(entries: appState.todayEntries)

                    QuickActionsView()
                }
                .padding()
            }

            if let error = appState.errorMessage {
                Text(error)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal)
                    .padding(.bottom, 8)
            }
        }
        .frame(width: 380, height: 520)
        .task {
            await appState.refresh()
        }
    }
}

private struct ServerStatusBadge: View {
    let state: ServerState
    let port: Int

    var body: some View {
        HStack(spacing: 4) {
            Circle()
                .fill(dotColor)
                .frame(width: 6, height: 6)
            Text(statusText)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .padding(.trailing, 8)
    }

    private var dotColor: Color {
        switch state {
        case .running: .green
        case .starting: .yellow
        case .stopped: .red
        case .error: .red
        }
    }

    private var statusText: String {
        switch state {
        case .running: "Server :\(port)"
        case .starting: "Starting..."
        case .stopped: "Server off"
        case .error: "Server error"
        }
    }
}

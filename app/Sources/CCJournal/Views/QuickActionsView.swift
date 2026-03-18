import SwiftUI

struct QuickActionsView: View {
    @Environment(AppState.self) private var appState
    @State private var standupCopied = false
    @State private var weeklyCopied = false

    private let cli = CLIBridge()

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Quick Actions")
                .font(.subheadline.bold())
                .foregroundStyle(.secondary)

            HStack(spacing: 8) {
                ActionButton(
                    title: standupCopied ? "Copied!" : "Copy standup",
                    icon: standupCopied ? "checkmark" : "doc.on.clipboard",
                    shortcut: "S"
                ) {
                    await copyStandup()
                }

                ActionButton(
                    title: weeklyCopied ? "Copied!" : "Copy weekly",
                    icon: weeklyCopied ? "checkmark" : "doc.on.clipboard",
                    shortcut: "W"
                ) {
                    await copyWeekly()
                }
            }

            HStack(spacing: 8) {
                ActionButton(
                    title: "Open dashboard",
                    icon: "globe",
                    shortcut: "D"
                ) {
                    appState.serverManager.openDashboard()
                }

                ActionButton(
                    title: "Browse entries",
                    icon: "book",
                    shortcut: "B"
                ) {
                    appState.serverManager.openDashboard(path: "/browse")
                }
            }

            HStack(spacing: 8) {
                ActionButton(
                    title: appState.serverManager.state.isRunning ? "Stop server" : "Start server",
                    icon: appState.serverManager.state.isRunning ? "stop.fill" : "play.fill",
                    shortcut: "R"
                ) {
                    appState.serverManager.toggleServer()
                }
            }
        }
    }

    private func copyStandup() async {
        try? await cli.copyStandup()
        standupCopied = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
            standupCopied = false
        }
    }

    private func copyWeekly() async {
        try? await cli.copyWeekly()
        weeklyCopied = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
            weeklyCopied = false
        }
    }
}

private struct ActionButton: View {
    let title: String
    let icon: String
    let shortcut: String
    let action: () async -> Void

    var body: some View {
        Button {
            Task { await action() }
        } label: {
            HStack(spacing: 4) {
                Image(systemName: icon)
                Text(title)
                    .lineLimit(1)
                Spacer()
                Text("\u{2318}\(shortcut)")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
            .font(.caption)
            .padding(.vertical, 6)
            .padding(.horizontal, 8)
        }
        .buttonStyle(.borderless)
        .frame(maxWidth: .infinity)
        .background(.quaternary.opacity(0.3))
        .clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

import SwiftUI

struct EntryRow: View {
    let entry: JournalEntry
    @State private var isExpanded = false
    @State private var showCopied = false

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 6) {
                Circle()
                    .fill(entry.hasAiSummary ? .green : .yellow)
                    .frame(width: 8, height: 8)

                Text("\(entry.project)")
                    .font(.callout.bold()) +
                Text(" (\(entry.branch))")
                    .font(.callout)
                    .foregroundColor(.secondary)

                Spacer()

                Text(entry.timeRange)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Text(isExpanded ? entry.summary : String(entry.summary.prefix(80)))
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(isExpanded ? nil : 2)

            HStack {
                Spacer()
                Button {
                    copyResumeCommand()
                } label: {
                    Label(showCopied ? "Copied!" : "Resume", systemImage: showCopied ? "checkmark" : "play.fill")
                        .font(.caption)
                }
                .buttonStyle(.borderless)
                .tint(.accentColor)
            }
        }
        .padding(8)
        .contentShape(Rectangle())
        .onTapGesture {
            withAnimation(.easeInOut(duration: 0.2)) {
                isExpanded.toggle()
            }
        }
    }

    private func copyResumeCommand() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(entry.resumeCommand, forType: .string)
        showCopied = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
            showCopied = false
        }
    }
}

import Foundation
import os.log

private let logger = Logger(subsystem: "com.ccjournal.app", category: "FileWatcher")

final class FileWatcher {
    private let directory: String
    private let onChange: () -> Void
    private var source: DispatchSourceFileSystemObject?
    private var fileDescriptor: Int32 = -1

    init(directory: String, onChange: @escaping () -> Void) {
        self.directory = directory
        self.onChange = onChange
    }

    deinit {
        stop()
    }

    func start() {
        fileDescriptor = open(directory, O_EVTONLY)
        guard fileDescriptor >= 0 else {
            logger.error("Failed to open directory for watching: \(self.directory)")
            return
        }

        logger.info("Watching \(self.directory) for changes")

        let source = DispatchSource.makeFileSystemObjectSource(
            fileDescriptor: fileDescriptor,
            eventMask: [.write, .rename, .extend],
            queue: .main
        )

        source.setEventHandler { [weak self] in
            logger.debug("File change detected in journal directory")
            self?.onChange()
        }

        source.setCancelHandler { [weak self] in
            guard let self, self.fileDescriptor >= 0 else { return }
            close(self.fileDescriptor)
            self.fileDescriptor = -1
        }

        source.resume()
        self.source = source
    }

    func stop() {
        logger.info("Stopping file watcher")
        source?.cancel()
        source = nil
    }
}

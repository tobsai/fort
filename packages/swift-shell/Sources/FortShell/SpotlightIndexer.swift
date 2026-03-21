import Foundation

// TODO: Import CoreSpotlight and MobileCoreServices when building with Xcode
// These frameworks are not available in command-line Swift builds
// #if canImport(CoreSpotlight)
// import CoreSpotlight
// import MobileCoreServices  // provides UTType constants (kUTTypeText, etc.)
// #endif

/// Indexes Fort tasks, threads, and memory nodes into macOS Spotlight
/// so users can find Fort content via Spotlight search (Cmd+Space).
///
/// TODO: Wire up to CoreSpotlight APIs:
///   - Use CSSearchableIndex.default() for indexing
///   - Create CSSearchableItemAttributeSet with appropriate content types
///   - Use CSSearchableItem with domainIdentifier "com.fort.shell"
///   - Handle CSSearchableIndex.default().deleteSearchableItems for removal
///   - Implement indexDelegate for on-demand indexing via CSIndexExtensionRequestHandler
///   - Register the app's CSSearchableIndex in Info.plist
class SpotlightIndexer {

    /// Represents an indexed item stored locally until CoreSpotlight is wired up
    struct IndexedItem {
        let id: String
        let title: String
        let description: String
        let itemType: String  // "task", "thread", or "memory"
    }

    /// Local store of indexed items (placeholder for CSSearchableIndex)
    private(set) var indexedItems: [IndexedItem] = []

    private let domainIdentifier = "com.fort.shell"

    init() {
        print("[FortShell:SpotlightIndexer] Initialized (stub — CoreSpotlight not yet wired)")
    }

    // MARK: - Indexing

    /// Index a Fort task into Spotlight
    ///
    /// TODO: Create CSSearchableItemAttributeSet with:
    ///   - contentType: UTType.text
    ///   - title, contentDescription, keywords (agent name, status)
    ///   - Create CSSearchableItem with uniqueIdentifier "task:\(id)"
    ///   - Call CSSearchableIndex.default().indexSearchableItems([item])
    func indexTask(id: String, title: String, description: String, status: String, agent: String?) {
        print("[FortShell:SpotlightIndexer] indexTask (stub) — id=\(id) title=\(title) status=\(status) agent=\(agent ?? "none")")

        let item = IndexedItem(
            id: "task:\(id)",
            title: title,
            description: "\(description) [status: \(status)]",
            itemType: "task"
        )
        indexedItems.removeAll { $0.id == item.id }
        indexedItems.append(item)
    }

    /// Index a Fort thread (conversation) into Spotlight
    ///
    /// TODO: Create CSSearchableItemAttributeSet with:
    ///   - contentType: UTType.text
    ///   - title: thread name, contentDescription: lastMessage snippet
    ///   - Create CSSearchableItem with uniqueIdentifier "thread:\(id)"
    func indexThread(id: String, name: String, lastMessage: String?) {
        print("[FortShell:SpotlightIndexer] indexThread (stub) — id=\(id) name=\(name)")

        let item = IndexedItem(
            id: "thread:\(id)",
            title: name,
            description: lastMessage ?? "",
            itemType: "thread"
        )
        indexedItems.removeAll { $0.id == item.id }
        indexedItems.append(item)
    }

    /// Index a Fort memory node into Spotlight
    ///
    /// TODO: Create CSSearchableItemAttributeSet with:
    ///   - contentType: UTType.text
    ///   - title: label, keywords: [type]
    ///   - Create CSSearchableItem with uniqueIdentifier "memory:\(id)"
    func indexMemory(id: String, label: String, type: String) {
        print("[FortShell:SpotlightIndexer] indexMemory (stub) — id=\(id) label=\(label) type=\(type)")

        let item = IndexedItem(
            id: "memory:\(id)",
            title: label,
            description: "Memory node [\(type)]",
            itemType: "memory"
        )
        indexedItems.removeAll { $0.id == item.id }
        indexedItems.append(item)
    }

    // MARK: - Removal

    /// Remove a single item from the Spotlight index
    ///
    /// TODO: Call CSSearchableIndex.default().deleteSearchableItems(withIdentifiers: [id])
    func removeItem(id: String) {
        print("[FortShell:SpotlightIndexer] removeItem (stub) — id=\(id)")
        indexedItems.removeAll { $0.id == id }
    }

    // MARK: - Reindex

    /// Perform a full reindex — called on startup to ensure Spotlight is in sync
    ///
    /// TODO:
    ///   1. Call CSSearchableIndex.default().deleteAllSearchableItems() first
    ///   2. Fetch all tasks, threads, and memory nodes from Fort core via WebSocket
    ///   3. Re-index everything in batches (CSSearchableIndex supports batch indexing)
    ///   4. Log completion count
    func reindexAll() {
        print("[FortShell:SpotlightIndexer] reindexAll (stub) — would clear and rebuild entire index")
        print("[FortShell:SpotlightIndexer] Currently holding \(indexedItems.count) items in local stub array")

        // TODO: Wire to WebSocket to fetch all indexable items from Fort core
        // webSocketClient.send(action: "spotlight_reindex_request")
    }
}

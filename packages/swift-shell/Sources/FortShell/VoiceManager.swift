import AppKit

// TODO: Import Speech and AVFoundation when building with Xcode
// These frameworks may not be fully available in command-line Swift builds
// #if canImport(Speech)
// import Speech
// #endif
// #if canImport(AVFoundation)
// import AVFoundation
// #endif

/// Manages voice input (speech recognition) and output (text-to-speech) for Fort
///
/// TODO: Microphone and Speech Recognition permissions are REQUIRED:
///   1. Add NSMicrophoneUsageDescription to Info.plist
///   2. Add NSSpeechRecognitionUsageDescription to Info.plist
///   3. Call SFSpeechRecognizer.requestAuthorization() on first use
///   4. Call AVCaptureDevice.requestAccess(for: .audio) for microphone
///   5. Handle all authorization states: .authorized, .denied, .restricted, .notDetermined
///
/// TODO: Speech framework setup:
///   - Create SFSpeechRecognizer(locale: Locale(identifier: "en-US"))
///   - Use SFSpeechAudioBufferRecognitionRequest for streaming recognition
///   - Configure AVAudioEngine to capture microphone input
///   - Feed audio buffers to the recognition request
///   - Handle SFSpeechRecognitionResult for interim and final results
///
/// TODO: Text-to-speech setup:
///   - Create AVSpeechSynthesizer
///   - Create AVSpeechUtterance with desired text
///   - Configure voice via AVSpeechSynthesisVoice(language: "en-US")
///   - Optionally use AVSpeechSynthesisVoice(identifier:) for a specific voice
///   - Set rate, pitchMultiplier, volume on the utterance
class VoiceManager {

    /// Whether speech recognition is currently active
    private(set) var isListening: Bool = false

    /// Callback invoked when speech is transcribed
    var onTranscription: ((String) -> Void)?

    /// Whether speech recognition is available on this system
    private(set) var isAvailable: Bool = false

    // TODO: Replace with actual framework objects:
    // private var speechRecognizer: SFSpeechRecognizer?
    // private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    // private var recognitionTask: SFSpeechRecognitionTask?
    // private var audioEngine: AVAudioEngine?
    // private var synthesizer: AVSpeechSynthesizer?

    init() {
        print("[FortShell:VoiceManager] Initialized (stub — Speech/AVFoundation not yet wired)")

        // TODO: Check actual availability:
        // isAvailable = SFSpeechRecognizer()?.isAvailable ?? false
        // if !isAvailable {
        //     print("[FortShell:VoiceManager] Speech recognition not available on this system")
        // }
    }

    // MARK: - Speech Recognition

    /// Begin listening for speech input via the microphone
    ///
    /// TODO: Implementation steps:
    ///   1. Check SFSpeechRecognizer.authorizationStatus()
    ///   2. If .notDetermined, call SFSpeechRecognizer.requestAuthorization()
    ///   3. Create SFSpeechAudioBufferRecognitionRequest
    ///   4. Set up AVAudioEngine with an input node tap:
    ///      let inputNode = audioEngine.inputNode
    ///      let format = inputNode.outputFormat(forBus: 0)
    ///      inputNode.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in
    ///          recognitionRequest.append(buffer)
    ///      }
    ///   5. Start audioEngine: audioEngine.prepare(); try audioEngine.start()
    ///   6. Start recognition task:
    ///      recognitionTask = speechRecognizer.recognitionTask(with: request) { result, error in
    ///          if let result = result {
    ///              let text = result.bestTranscription.formattedString
    ///              if result.isFinal { self.onTranscription?(text) }
    ///          }
    ///      }
    func startListening() {
        guard !isListening else {
            print("[FortShell:VoiceManager] Already listening")
            return
        }

        print("[FortShell:VoiceManager] startListening (stub) — would begin speech recognition")
        isListening = true

        // TODO: Start AVAudioEngine and SFSpeechRecognitionTask
        // Simulate a transcription for testing:
        // DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
        //     self.onTranscription?("Test transcription — not yet implemented")
        // }
    }

    /// Stop listening for speech input
    ///
    /// TODO: Implementation steps:
    ///   1. audioEngine?.stop()
    ///   2. audioEngine?.inputNode.removeTap(onBus: 0)
    ///   3. recognitionRequest?.endAudio()
    ///   4. recognitionTask?.cancel()
    ///   5. Clean up references
    func stopListening() {
        guard isListening else {
            print("[FortShell:VoiceManager] Not currently listening")
            return
        }

        print("[FortShell:VoiceManager] stopListening (stub) — would stop speech recognition")
        isListening = false

        // TODO: Stop AVAudioEngine and recognition task
    }

    // MARK: - Text-to-Speech

    /// Speak the given text aloud using system text-to-speech
    ///
    /// TODO: Implementation steps:
    ///   1. Create AVSpeechUtterance(string: text)
    ///   2. Set utterance.voice = AVSpeechSynthesisVoice(language: "en-US")
    ///   3. Optionally set utterance.rate, utterance.pitchMultiplier
    ///   4. Call synthesizer.speak(utterance)
    ///   5. Implement AVSpeechSynthesizerDelegate for completion callbacks
    func speak(text: String) {
        print("[FortShell:VoiceManager] speak (stub) — would say: \"\(text)\"")

        // TODO: Use NSSpeechSynthesizer (macOS) or AVSpeechSynthesizer:
        // let utterance = AVSpeechUtterance(string: text)
        // utterance.voice = AVSpeechSynthesisVoice(language: "en-US")
        // synthesizer?.speak(utterance)
    }

    deinit {
        if isListening {
            stopListening()
        }
    }
}

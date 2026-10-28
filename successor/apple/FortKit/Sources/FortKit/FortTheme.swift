// Generated from successor/design/tokens.json by `node successor/design/tokens.mjs --write`. Edit the JSON, not this file.
import SwiftUI

/// Fort's comic skin. The helpers that draw with it are in FortThemeSupport.swift.
public enum FortTheme {
    /// The page behind every panel.
    public static let paper = Color(light:0xFFF8EA,dark:0x16151E)
    /// A panel, card or bubble.
    public static let panel = Color(light:0xFFFDF6,dark:0x22202D)
    /// Header, composer and sheet bars.
    public static let bar = Color(light:0xFFF1D6,dark:0x2A2738)
    /// Body text.
    public static let ink = Color(light:0x1B1A24,dark:0xF7F0E1)
    /// Panel outlines and their hard offset shadows.
    public static let line = Color(light:0x1B1A24,dark:0x000000)
    /// Secondary text.
    public static let muted = Color(light:0x5B5866,dark:0xB7B0C2)
    /// Links and quiet buttons.
    public static let link = Color(light:0x0B6E7A,dark:0x7FE7F2)
    /// Your own speech bubbles.
    public static let mine = Color(light:0xDDF7FA,dark:0x1F3A40)
    /// Caption boxes: section labels and status lines. Text on it is always captionInk.
    public static let caption = Color(hex:0xFFE7A0)
    /// Text on caption boxes, gold and status fills.
    public static let captionInk = Color(hex:0x1B1A24)
    /// Quest gold: the one primary action and what needs you.
    public static let accent = Color(hex:0xF6C25B)
    /// Status: an agent is working. Holographic cyan.
    public static let working = Color(light:0x7FE7F2,dark:0x3FE0F0)
    /// The holographic glow around live or selected things.
    public static let glow = Color(hex:0x3FE0F0)
    /// Status: waiting on you. Same gold as the accent.
    public static let needsYou = Color(hex:0xF6C25B)
    /// Status: finished. The Quest clear stamp.
    public static let done = Color(hex:0x8FD47E)
    /// Status: failed. White text on it.
    public static let failed = Color(hex:0xE2574C)
    /// Status: stopped, canceled or offline.
    public static let off = Color(light:0xB9B3A6,dark:0x56617A)
    /// Portrait colours for the crew; FortTheme.crewColor(for:) picks one per Agent.
    public static let crew: [Color] = [Color(hex:0xA9DCEF),Color(hex:0xF9C79A),Color(hex:0xB9E3A6),Color(hex:0xF6B8C8),Color(hex:0xD9C2F0),Color(hex:0xF6D57A)]
    /// The golden-hour sky behind the top bar, left to right.
    public static let sky: [Color] = [Color(hex:0xF6B8C8),Color(hex:0xF9C79A),Color(hex:0xF6D57A),Color(hex:0xA9DCEF)]
    public static let stroke: CGFloat = 2.5
    public static let radius: CGFloat = 8
    public static let bubbleRadius: CGFloat = 18
    public static let shadow: CGFloat = 4
    public static let touch: CGFloat = 44
    public static let displayFont = "AvenirNextCondensed-Heavy"
}

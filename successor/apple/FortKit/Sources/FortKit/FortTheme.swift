// Generated from successor/design/tokens.json by `node successor/design/tokens.mjs --write`. Edit the JSON, not this file.
import SwiftUI

/// Fort's skin. The helpers that draw with it are in FortThemeSupport.swift.
public enum FortTheme {
    /// The page behind every panel.
    public static let paper = Color(light:0xF2F4F7,dark:0x0B0E13)
    /// A panel, card or bubble.
    public static let panel = Color(light:0xFFFFFF,dark:0x131820)
    /// Header, composer and sheet bars.
    public static let bar = Color(light:0xE8ECF1,dark:0x0F1319)
    /// Body text.
    public static let ink = Color(light:0x0E1218,dark:0xE8ECF1)
    /// Hairline edges of panels, bars and bubbles.
    public static let line = Color(light:0xC6CDD7,dark:0x283039)
    /// Secondary text.
    public static let muted = Color(light:0x545E6C,dark:0x8C97A6)
    /// Links and quiet buttons.
    public static let link = Color(light:0x0A6874,dark:0x5FD4E0)
    /// Your own speech bubbles.
    public static let mine = Color(light:0xDDF0F4,dark:0x132C34)
    /// Tags: section labels and status lines. Text on it is ink.
    public static let caption = Color(light:0xE3E8EE,dark:0x1A212B)
    /// Text on gold, cyan and green fills.
    public static let captionInk = Color(hex:0x0B0E13)
    /// Quest gold: the one primary action and what needs you.
    public static let accent = Color(hex:0xF6C25B)
    /// Status: an agent is working. Holographic cyan.
    public static let working = Color(light:0x6FDDEA,dark:0x3FE0F0)
    /// The holographic glow around live or selected things.
    public static let glow = Color(hex:0x3FE0F0)
    /// Status: waiting on you. Same gold as the accent.
    public static let needsYou = Color(hex:0xF6C25B)
    /// Status: finished.
    public static let done = Color(hex:0x7CD3A0)
    /// Status: failed. White text on it.
    public static let failed = Color(hex:0xE2574C)
    /// Status: stopped, canceled or offline.
    public static let off = Color(light:0xA3ACB8,dark:0x4A5462)
    /// Portrait colours for the crew; FortTheme.crewColor(for:) picks one per Agent.
    public static let crew: [Color] = [Color(hex:0x7FB6D3),Color(hex:0xD8A27C),Color(hex:0x8DC4A0),Color(hex:0xD493A9),Color(hex:0xA79BD6),Color(hex:0xD3BE7E)]
    /// The night behind the top bar, left to right. The same in light and dark.
    public static let sky: [Color] = [Color(hex:0x0B0E13),Color(hex:0x141B25),Color(hex:0x0B0E13)]
    public static let stroke: CGFloat = 1
    public static let radius: CGFloat = 4
    public static let bubbleRadius: CGFloat = 6
    public static let shadow: CGFloat = 0
    public static let touch: CGFloat = 44
    public static let displayFont = "AvenirNextCondensed-Heavy"
}

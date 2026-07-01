//
//  FortComplicationBundle.swift
//  FortComplication (watchOS widget extension)
//
//  The @main entry point for the complication's widget extension. The widget
//  itself (FortComplication) lives in ../watch/FortComplication.swift; this
//  bundle is what the extension target runs.
//

import WidgetKit
import SwiftUI

@main
struct FortComplicationBundle: WidgetBundle {
    var body: some Widget {
        FortComplication()
    }
}

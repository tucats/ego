//
// TableCollection.swift
//
// Generated by openapi-generator
// https://openapi-generator.tech
//

import Foundation
#if canImport(AnyCodable)
import AnyCodable
#endif

public struct TableCollection: Codable, Hashable {

    /** An array of the table name information */
    public var tables: [Table]
    /** The number of items in the table array */
    public var count: Int

    public init(tables: [Table], count: Int) {
        self.tables = tables
        self.count = count
    }

    public enum CodingKeys: String, CodingKey, CaseIterable {
        case tables
        case count
    }

    // Encodable protocol methods

    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(tables, forKey: .tables)
        try container.encode(count, forKey: .count)
    }
}


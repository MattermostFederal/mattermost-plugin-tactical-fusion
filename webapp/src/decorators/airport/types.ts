/**
 * One airfield, already rendered.
 *
 * Strings rather than fields, for the reason `Conversion` is strings: the
 * rendering rules live in Go and a second implementation of them here is a
 * second thing to get wrong.
 *
 * Mirrors `airportDetails` in `server/api.go`, and is held to it by
 * `TestWebappAirportShapeMatches` on names, types and order.
 */
export interface AirportDetails {
    name: string;
    type: string;
    place: string;

    /**
     * Feet, already formatted, or empty when the database states none.
     *
     * Empty is "not stated", which is a different thing from sea level: sea
     * level renders "0 ft". Nothing may collapse the two, which is the
     * truthiness defect this plugin did not inherit from the sibling plugin.
     */
    elevation: string;

    iata: string;
}

/**
 * The location decorator's own link identity for this airfield's position.
 *
 * A pair rather than a set of readings, because an airfield surface links to
 * the coordinate instead of reprinting it. Built in Go rather than here: the
 * two languages disagree about formatting a float in the last digit, and a
 * token that differs by one digit is refused by the very view it opens.
 *
 * Mirrors `airportCoordinate` in `server/api.go`.
 */
export interface AirportCoordinate {
    format: string;
    value: string;

    /**
     * The country, carrying its own citation.
     *
     * It reaches the map's accessible label and nothing else, which is the only
     * place the country reaches a reader who is not looking at the map. Unlike
     * the two above it may legitimately be empty: a position over open ocean is
     * in no country, and reporting that as an outage would be a lie.
     */
    region: string;
}

/**
 * What `/api/v1/airport` answers.
 *
 * Discriminated on `found`. An ident this build does not hold carries no
 * airfield and no coordinate at all rather than zero values a reader would take
 * for real: 0,0 is a position like any other, and this plugin deliberately does
 * not inherit the truthiness check that drops it.
 *
 * Mirrors `airportResponse` in `server/api.go`.
 */
export interface AirportResponse {
    found: boolean;
    ident: string;
    airport?: AirportDetails;
    coordinate?: AirportCoordinate;
}

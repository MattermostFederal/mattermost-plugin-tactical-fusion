export interface AirportEnd {
    format: string;
    value: string;
}

export interface AirportRunway {
    designation: string;
    summary: string;
    length: string;
    width: string;
    surface: string;
    lighted: string;
    closed: string;
    ends?: [AirportEnd, AirportEnd];
}

export interface AirportFrequency {
    type: string;
    description: string;
    mhz: string;
}

export interface AirportDetails {
    name: string;
    type: string;
    place: string;
    elevation: string;
    iata: string;
    military: string;
    runways: AirportRunway[];
    frequencies: AirportFrequency[];
}

export interface AirportCoordinate {
    format: string;
    value: string;
    region: string;
}

export interface AirportResponse {
    found: boolean;
    ident: string;
    iata: string;
    airport?: AirportDetails;
    coordinate?: AirportCoordinate;
}

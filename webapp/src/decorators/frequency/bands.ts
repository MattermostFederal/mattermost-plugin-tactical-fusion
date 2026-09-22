export interface Band {
    low: number;
    high: number;
    name: string;
}

export const BANDS: readonly Band[] = [
    {low: 2000, high: 30000, name: 'HF aeronautical'},
    {low: 108000, high: 117975, name: 'VHF navigation'},
    {low: 118000, high: 136975, name: 'VHF air band'},
    {low: 156000, high: 162025, name: 'VHF marine'},
    {low: 225000, high: 400000, name: 'UHF military air band'},
    {low: 406000, high: 406100, name: 'Distress beacons'},
];

export interface Allocation {
    khz: number;
    use: string;
}

export const ALLOCATIONS: readonly Allocation[] = [
    {khz: 121500, use: 'Aeronautical emergency'},
    {khz: 123100, use: 'Search and rescue on-scene'},
    {khz: 122750, use: 'Air-to-air, fixed wing'},
    {khz: 123450, use: 'Air-to-air'},
    {khz: 243000, use: 'UHF military emergency'},
    {khz: 156800, use: 'Marine channel 16, distress and calling'},
    {khz: 406000, use: 'Distress beacon, COSPAS-SARSAT'},
];

export const OUTSIDE_BANDS = 'Outside the aviation bands this plugin names';

const VHF_AIR_LOW = 118000;
const VHF_AIR_HIGH = 136975;

export function bandOf(khz: number): string {
    return BANDS.find((band) => khz >= band.low && khz <= band.high)?.name ?? OUTSIDE_BANDS;
}

export function channelOf(khz: number): string {
    if (khz < VHF_AIR_LOW || khz > VHF_AIR_HIGH) {
        return '';
    }
    return khz % 25 === 0 ? '25 kHz channel' : '8.33 kHz channel';
}

export function allocationOf(khz: number): string {
    return ALLOCATIONS.find((allocation) => allocation.khz === khz)?.use ?? '';
}

export function mhzText(khz: number): string {
    return `${Math.floor(khz / 1000)}.${String(khz % 1000).padStart(3, '0')}`;
}

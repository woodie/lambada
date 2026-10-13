const SIZE_UNITS = ['KB', 'MB', 'GB', 'TB', 'PB', 'EB'];
// Lines up with SIZE_UNITS: KB 0, MB 1, GB and above 2.
const DECIMAL_PLACES = [0, 1, 2, 2, 2, 2];
export function humanSize(bytes) {
    if (bytes === 0)
        return 'Zero KB';
    if (bytes < 1000)
        return bytes === 1 ? '1 byte' : `${bytes} bytes`;
    let exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1000)), SIZE_UNITS.length);
    let value = roundForUnit(bytes, exp);
    if (value >= 1000 && exp < SIZE_UNITS.length) {
        exp++;
        value = roundForUnit(bytes, exp);
    }
    return `${trimZeros(value, DECIMAL_PLACES[exp - 1])} ${SIZE_UNITS[exp - 1]}`;
}
function roundForUnit(bytes, exp) {
    const scale = 10 ** DECIMAL_PLACES[exp - 1];
    return Math.round(bytes / 1000 ** exp * scale) / scale;
}
function trimZeros(value, decimals) {
    const s = value.toFixed(decimals);
    return s.includes('.') ? s.replace(/0+$/, '').replace(/\.$/, '') : s;
}

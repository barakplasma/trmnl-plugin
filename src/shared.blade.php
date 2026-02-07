@php
use Carbon\Carbon;

$kids = [];
for ($i = 1; $i <= 4; $i++) {
    $name = $config["kid_{$i}_name"] ?? '';
    $birthday = $config["kid_{$i}_birthday"] ?? '';
    if (!empty(trim($name)) && !empty(trim($birthday))) {
        try {
            $bday = Carbon::parse(trim($birthday));
            $now = Carbon::now();
            $diff = $bday->diff($now);
            $kids[] = [
                'name' => trim($name),
                'birthday' => $bday,
                'now' => $now,
                'diff' => $diff,
            ];
        } catch (\Exception $e) {
            // skip invalid dates
        }
    }
}
@endphp

<?php

use Carbon\Carbon;

// This file handles data computation for the Kids Age plugin
// It processes the configuration and calculates ages

class KidsAgePlugin {
    private $config;

    public function __construct($config) {
        $this->config = $config ?? [];
    }

    /**
     * Get computed kids data for template rendering
     */
    public function getKids() {
        $kids = [];

        for ($i = 1; $i <= 4; $i++) {
            $nameKey = "kid_{$i}_name";
            $birthdayKey = "kid_{$i}_birthday";

            $name = $this->config[$nameKey] ?? '';
            $birthday = $this->config[$birthdayKey] ?? '';

            if (!empty(trim($name)) && !empty(trim($birthday))) {
                try {
                    $birthDate = Carbon::parse(trim($birthday));
                    $today = Carbon::now();
                    $diff = $birthDate->diff($today);

                    $kids[] = [
                        'name' => trim($name),
                        'birthday' => $birthDate,
                        'today' => $today,
                        'years' => $diff->y,
                        'months' => $diff->m,
                        'days' => $diff->d,
                        'birthday_formatted' => $birthDate->format('l F j, Y'),
                        'today_formatted' => $today->format('l F j, Y'),
                        'birthday_short' => $birthDate->format('M j, Y'),
                    ];
                } catch (\Exception $e) {
                    // Skip invalid dates
                    continue;
                }
            }
        }

        return $kids;
    }
}

// Return the computed data
$plugin = new KidsAgePlugin($config ?? []);
return [
    'kids' => $plugin->getKids(),
];

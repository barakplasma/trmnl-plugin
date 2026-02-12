@php
    // Include plugin logic to get kids data
    $plugin_data = include __DIR__ . '/plugin.php';
    $kids = $plugin_data['kids'] ?? [];
@endphp

@if (count($kids) > 0)
<div class="layout layout--col gap--space-between h--full">
    <div class="layout layout--col gap--large">
        @foreach ($kids as $kid)
        <div class="layout layout--col gap--medium">
            <span class="value value--medium">{{ $kid['name'] }}</span>
            <div class="layout layout--col">
                <span class="label">Born on: {{ $kid['birthday_formatted'] }}</span>
                <span class="label">Age today: {{ $kid['today_formatted'] }}</span>
            </div>
            <span class="value">= {{ $kid['years'] }} years {{ $kid['months'] }} months {{ $kid['days'] }} days</span>
        </div>
        @if (!$loop->last)
        <div class="b-h-gray-1"></div>
        @endif
        @endforeach
    </div>
</div>
@else
<div class="layout layout--col justify-center h--full">
    <span class="label">Configure kids names and birthdays in plugin settings</span>
</div>
@endif

<div class="title_bar">
    <span class="title">Kids Age</span>
</div>

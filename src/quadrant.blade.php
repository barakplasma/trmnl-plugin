@php
    $plugin_data = include __DIR__ . '/plugin.php';
    $kids = $plugin_data['kids'] ?? [];
@endphp

@if (count($kids) > 0)
<div class="layout layout--col gap--medium justify-center h--full">
    @foreach ($kids as $kid)
    <div class="layout layout--col">
        <span class="value value--small">{{ $kid['name'] }}</span>
        <span class="label">{{ $kid['years'] }}y {{ $kid['months'] }}m {{ $kid['days'] }}d</span>
    </div>
    @endforeach
</div>
@else
<div class="layout layout--col justify-center h--full">
    <span class="label">Configure in settings</span>
</div>
@endif

<div class="title_bar">
    <span class="title">Kids Age</span>
</div>

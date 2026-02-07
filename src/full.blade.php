@if (count($kids) > 0)
<div class="layout layout--col gap--space-between h--full">
    <div class="layout layout--col gap--large">
        @foreach ($kids as $kid)
        <div class="layout layout--col gap--medium">
            <span class="value value--medium">{{ $kid['name'] }}</span>
            <div class="layout layout--col">
                <span class="label">Born on: {{ $kid['birthday']->format('l F j, Y') }}</span>
                <span class="label">Age today: {{ $kid['now']->format('l F j, Y') }}</span>
            </div>
            <span class="value">= {{ $kid['diff']->y }} years {{ $kid['diff']->m }} months {{ $kid['diff']->d }} days</span>
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

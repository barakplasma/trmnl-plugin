@if (count($kids) > 0)
<div class="layout layout--col gap--space-between h--full">
    @foreach ($kids as $kid)
    <div class="layout layout--col gap--medium">
        <span class="value value--medium">{{ $kid['name'] }}</span>
        <div class="layout layout--col">
            <span class="label">Born: {{ $kid['birthday']->format('M j, Y') }}</span>
            <span class="label">Today: {{ $kid['now']->format('M j, Y') }}</span>
        </div>
        <span class="value value--small">= {{ $kid['diff']->y }} years {{ $kid['diff']->m }} months {{ $kid['diff']->d }} days</span>
    </div>
    @if (!$loop->last)
    <div class="b-h-gray-1"></div>
    @endif
    @endforeach
</div>
@else
<div class="layout layout--col justify-center h--full">
    <span class="label">Configure kids in plugin settings</span>
</div>
@endif

<div class="title_bar">
    <span class="title">Kids Age</span>
</div>

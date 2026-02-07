@if (count($kids) > 0)
<div class="layout layout--row gap--large h--full">
    @foreach ($kids as $kid)
    <div class="layout layout--col gap--medium justify-center">
        <span class="value value--medium">{{ $kid['name'] }}</span>
        <span class="label">Born: {{ $kid['birthday']->format('M j, Y') }}</span>
        <span class="value value--small">{{ $kid['diff']->y }}y {{ $kid['diff']->m }}m {{ $kid['diff']->d }}d</span>
    </div>
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

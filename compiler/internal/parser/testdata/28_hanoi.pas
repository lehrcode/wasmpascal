program TuermeVonHanoi;
var
  Zuege: LongInt;

procedure Bewege(n: Integer; von, nach, ueber: Char);
begin
  if n > 0 then
  begin
    Bewege(n - 1, von, ueber, nach);
    Inc(Zuege);
    WriteLn('Scheibe ', n, ': ', von, ' -> ', nach);
    Bewege(n - 1, ueber, nach, von)
  end
end;

begin
  Zuege := 0;
  Bewege(4, 'A', 'C', 'B');
  WriteLn(Zuege, ' Zuege')
end.

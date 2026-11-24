program IOFehler2;
var
  f: Text;
begin
  Assign(f, 'fehlt.txt');
  WriteLn('vor Reset');
  Reset(f);
  WriteLn('nie')
end.

program Verschachtelt;
var
  Global: Integer;

procedure Aussen(n: Integer);
var
  Summe: Integer;

  procedure Mitte(k: Integer);
  var
    Faktor: Integer;

    function Innen(x: Integer): Integer;
    begin
      Innen := x * Faktor + n
    end;

  begin
    Faktor := 2;
    Summe := Summe + Innen(k);
    if k > 0 then Mitte(k - 1)
  end;

begin
  Summe := 0;
  Mitte(n);
  Global := Summe
end;

begin
  Aussen(3);
  WriteLn(Global)
end.

program Verschachtelt;
var
  Global: Integer;

function Summe(n: Integer): Integer;
var
  Zaehler: Integer;

  procedure Addiere(k: Integer);
  begin
    if k > 0 then
    begin
      Zaehler := Zaehler + k;
      Addiere(k - 1)
    end
  end;

begin
  Zaehler := 0;
  Addiere(n);
  Summe := Zaehler
end;

procedure Aussen(Tiefe: Integer);
var
  Text: string;

  procedure Mitte(n: Integer);

    procedure Innen;
    begin
      Text := Text + Chr(Ord('0') + n);
      Inc(Global, Tiefe)
    end;

  begin
    Innen;
    if n > 1 then Mitte(n - 1)
  end;

begin
  Text := '';
  Mitte(Tiefe);
  WriteLn(Text)
end;

procedure Tausche(var a, b: Integer);

  procedure Hilfe;
  var
    h: Integer;
  begin
    h := a; a := b; b := h
  end;

begin
  Hilfe
end;

function Fib(n: Integer): Integer;

  procedure Setze(w: Integer);
  begin
    Fib := w
  end;

begin
  if n < 2 then Setze(n) else Setze(Fib(n - 1) + Fib(n - 2))
end;

function Ackermann(m, n: Integer): Integer;

  function A(m, n: Integer): Integer;
  begin
    if m = 0 then A := n + 1
    else if n = 0 then A := A(m - 1, 1)
    else A := A(m - 1, A(m, n - 1))
  end;

begin
  Ackermann := A(m, n)
end;

var
  x, y: Integer;
begin
  WriteLn(Summe(10));
  Global := 0;
  Aussen(4);
  WriteLn(Global);
  x := 1; y := 2;
  Tausche(x, y);
  WriteLn(x, ' ', y);
  WriteLn(Fib(15));
  WriteLn(Ackermann(2, 3))
end.

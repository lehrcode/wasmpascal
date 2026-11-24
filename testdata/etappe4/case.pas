program CaseAnweisung;
type
  TAmpel = (Rot, Gelb, Gruen);
var
  n: Integer;
  c: Char;
  a: TAmpel;

procedure Beschreibe(n: Integer);
begin
  Write(n:4, ': ');
  case n of
    0: WriteLn('null');
    1, 3, 5, 7, 9: WriteLn('ungerade Ziffer');
    2, 4, 6, 8: WriteLn('gerade Ziffer');
    10..99: WriteLn('zweistellig');
    -9..-1: WriteLn('negativ');
  else
    Write('sonst');
    WriteLn(' etwas')
  end
end;

begin
  Beschreibe(0); Beschreibe(7); Beschreibe(4); Beschreibe(42);
  Beschreibe(-3); Beschreibe(100); Beschreibe(-10);
  for c := 'a' to 'e' do
    case c of
      'a', 'e', 'i', 'o', 'u': Write(UpCase(c));
    else
      Write(c)
    end;
  WriteLn;
  a := Rot;
  repeat
    case a of
      Rot: WriteLn('Halt');
      Gelb: WriteLn('Achtung');
      Gruen: WriteLn('Fahren')
    end;
    if a = High(TAmpel) then a := Low(TAmpel) else a := Succ(a)
  until a = Rot;
  case 3 * 4 of
    12: WriteLn('zwoelf')
  end
end.

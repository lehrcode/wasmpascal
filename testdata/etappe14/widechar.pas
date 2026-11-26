program Breit;
{ WideChar: Konstanten, Vergleiche, case, Array-Index, Verkettung, Ausgabe,
  Typumwandlungen und die Falle s[1]. }
type
  TZeichen = #$100..#$17F;
var
  W, V: WideChar;
  C: Char;
  S: string;
  Zaehler: array[WideChar] of Byte;
  T: TZeichen;
const
  Euro: WideChar = '€';

procedure Zeige(X: WideChar);
begin
  Write('[', X, ']')
end;

begin
  W := 'ä';
  WriteLn('W = ', W, ' Ord = ', Ord(W));
  W := #$20AC;
  WriteLn('Euro: ', W, ' ', Ord(W), ' ', Ord(Euro));
  W := 'a'; Inc(W); V := Succ(W);
  WriteLn(W, V, Pred(V), ' ', Ord(Low(WideChar)), ' ', Ord(High(WideChar)));
  C := 'b';
  WriteLn(W = C, ' ', W < 'ä', ' ', 'ä' < Euro, ' ', W = 'b', ' ', Euro > W);
  W := 'ö';
  case W of
    'a'..'z': WriteLn('klein');
    'ä', 'ö', 'ü': WriteLn('Umlaut');
    #$20AC: WriteLn('Euro');
  else
    WriteLn('sonst')
  end;
  Zaehler['ä'] := 3;
  Zaehler[Euro] := 7;
  WriteLn(Zaehler['ä'] + Zaehler[#$20AC]);
  S := 'Preis: ' + Euro + '5';
  WriteLn(S, ' ', Length(S));
  S := W;
  WriteLn(S, ' ', Length(S), ' ', Length(W));
  Write('|', W:3, '|', Euro:2, '|');
  WriteLn;
  Zeige('ü'); Zeige(C); Zeige(#$263A);
  WriteLn;
  W := WideChar($D800);
  WriteLn('Surrogat: ', W, ' ', Ord(W));
  WriteLn(SizeOf(WideChar), ' ', SizeOf(TZeichen));
  T := #$141;
  WriteLn(T, ' ', Ord(T));
  C := Char(Euro);
  WriteLn(Ord(C), ' ', Ord(WideChar(300)), ' ', Word(Euro));
  S := 'ä';
  W := S[1];
  WriteLn('s[1]: ', Ord(W));
  WriteLn(Length(#$E9), ' ', Length('é'));
  W := 'x'; V := W;
  WriteLn(W + V);
  WriteLn(Ord(WideChar($FFFE)), ' ', Ord(High(WideChar)) - Ord(WideChar($FFFE)))
end.
